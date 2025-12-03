package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type heartbeatPayload struct {
	Hostname      string              `json:"hostname,omitempty"`
	AgentVersion  string              `json:"agentVersion,omitempty"`
	DockerVersion string              `json:"dockerVersion,omitempty"`
	Platform      string              `json:"platform,omitempty"`
	Containers    []containerSnapshot `json:"containers,omitempty"`
	CPU           float64             `json:"cpu,omitempty"`
	Memory        float64             `json:"memory,omitempty"`
}

type containerSnapshot struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Image           string   `json:"image"`
	State           string   `json:"state"`
	Status          string   `json:"status"`
	AutoUpdate      bool     `json:"autoUpdate"`
	UpdateAvailable bool     `json:"updateAvailable"`
	Ports           []string `json:"ports,omitempty"`
	Labels          []string `json:"labels,omitempty"`
}

type agentCommand struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

func main() {
	var (
		serverURL  = envOrDefault("UPDOCKLY_SERVER", "")
		token      = envOrDefault("UPDOCKLY_AGENT_TOKEN", "")
		interval   = envOrDefaultDuration("UPDOCKLY_INTERVAL", 30*time.Second)
		agentName  = envOrDefault("UPDOCKLY_AGENT_NAME", "")
		userAgent  = "updockly-agent/0.1.0"
		dockerHost = os.Getenv("DOCKER_HOST")
		debug      = strings.EqualFold(envOrDefault("UPDOCKLY_DEBUG", "false"), "true")
		cmdPoll    = envOrDefaultDuration("UPDOCKLY_COMMAND_POLL", 5*time.Second)
		caCertPath = envOrDefault("UPDOCKLY_CA_CERT", "")
	)

	flag.StringVar(&serverURL, "server", serverURL, "Updockly server URL (e.g. https://updockly.example.com)")
	flag.StringVar(&token, "token", token, "Agent token issued by Updockly")
	flag.DurationVar(&interval, "interval", interval, "Heartbeat interval")
	flag.StringVar(&agentName, "name", agentName, "Agent name override (sent as hostname if provided)")
	flag.StringVar(&caCertPath, "ca-cert", caCertPath, "Path to trusted CA certificate file")
	flag.Parse()

	if serverURL == "" || token == "" {
		fmt.Println("UPDOCKLY_SERVER and UPDOCKLY_AGENT_TOKEN are required")
		os.Exit(1)
	}

	httpClient, err := createHTTPClient(caCertPath)
	if err != nil {
		fmt.Printf("failed to create http client: %v\n", err)
		os.Exit(1)
	}

	serverURL = strings.TrimRight(serverURL, "/")
	endpoint := serverURL + "/api/agents/heartbeat"
	commandBase := serverURL + "/api/agents"

	for {
		payload := gatherDockerInfo(agentName, dockerHost, userAgent)
		if err := sendHeartbeat(httpClient, endpoint, token, payload, userAgent); err != nil {
			fmt.Printf("heartbeat error: %v\n", err)
		}
		deadline := time.Now().Add(interval)
		for {
			if err := processCommands(httpClient, commandBase, token, dockerHost, userAgent, debug); err != nil {
				fmt.Printf("command processing error: %v\n", err)
			}
			if time.Now().After(deadline) {
				break
			}
			sleep := cmdPoll
			if sleep <= 0 {
				sleep = 3 * time.Second
			}
			time.Sleep(sleep)
		}
	}
}

func createHTTPClient(caCertPath string) (*http.Client, error) {
	if caCertPath == "" {
		return &http.Client{Timeout: 10 * time.Second}, nil
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ca cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append ca certs")
	}

	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}, nil
}

func gatherDockerInfo(agentName, dockerHost, userAgent string) heartbeatPayload {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := []client.Opt{
		client.FromEnv,
		client.WithUserAgent(userAgent),
		client.WithAPIVersionNegotiation(),
	}
	if strings.TrimSpace(dockerHost) != "" {
		opts = append(opts, client.WithHost(dockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return heartbeatPayload{Hostname: agentName}
	}
	defer cli.Close()

	ping, _ := cli.Ping(ctx)
	info, _ := cli.Info(ctx)

	hostname := agentName
	if hostname == "" && info.Name != "" {
		hostname = info.Name
	}

	dockerVersion := info.ServerVersion
	if dockerVersion == "" {
		dockerVersion = ping.APIVersion
	}
	if dockerVersion == "" {
		dockerVersion = "unknown"
	}

	platform := fmt.Sprintf("%s/%s", info.OSType, info.Architecture)
	if strings.Trim(platform, "/") == "" {
		platform = "unknown"
	}

	payload := heartbeatPayload{
		Hostname:      hostname,
		AgentVersion:  userAgent,
		DockerVersion: dockerVersion,
		Platform:      platform,
	}
	if hostname == "" {
		payload.Hostname = agentName
	}

	if c, err := cpu.Percent(0, false); err == nil && len(c) > 0 {
		payload.CPU = c[0]
	}
	if m, err := mem.VirtualMemory(); err == nil {
		payload.Memory = m.UsedPercent
	}

	if containers, err := cli.ContainerList(ctx, container.ListOptions{All: true}); err == nil {
		list := make([]containerSnapshot, 0, len(containers))
		for _, c := range containers {
			name := ""
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			var ports []string
			for _, p := range c.Ports {
				if p.PublicPort > 0 {
					ports = append(ports, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
				} else {
					ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
				}
			}
			var labels []string
			for k, v := range c.Labels {
				labels = append(labels, fmt.Sprintf("%s=%s", k, v))
			}
			list = append(list, containerSnapshot{
				ID:         c.ID,
				Name:       name,
				Image:      c.Image,
				State:      c.State,
				Status:     c.Status,
				AutoUpdate: false,
				Ports:      ports,
				Labels:     labels,
			})
		}
		payload.Containers = list
	}

	return payload
}

func sendHeartbeat(client *http.Client, endpoint, token string, payload heartbeatPayload, userAgent string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", token)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("heartbeat failed: %s", resp.Status)
	}
	return nil
}

func envOrDefault(key, def string) string {
	if val := strings.Trim(strings.TrimSpace(os.Getenv(key)), "\"'"); val != "" {
		return val
	}
	return def
}

func envOrDefaultDuration(key string, def time.Duration) time.Duration {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			return parsed
		}
	}
	return def
}

func processCommands(client *http.Client, baseURL, token, dockerHost, userAgent string, debug bool) error {
	for {
		cmd, err := fetchNextCommand(client, baseURL, token, userAgent, debug)
		if err != nil {
			return err
		}
		if cmd == nil {
			return nil
		}

		cid := containerIDFromPayload(cmd.Payload)
		if cid == "" {
			_ = reportCommand(client, baseURL, token, cmd.ID, "error", nil, "missing containerId", debug)
			continue
		}

		switch cmd.Type {
		case "check-update":
			available, err := runCheckUpdate(dockerHost, userAgent, cid)
			if err != nil {
				_ = reportCommand(client, baseURL, token, cmd.ID, "error", nil, err.Error(), debug)
				continue
			}
			result := map[string]interface{}{
				"containerId":     cid,
				"updateAvailable": available,
			}
			if err := reportCommand(client, baseURL, token, cmd.ID, "completed", result, "", debug); err != nil {
				return err
			}
		case "update-container":
			snapshot, err := runUpdateContainer(dockerHost, userAgent, cid)
			if err != nil {
				_ = reportCommand(client, baseURL, token, cmd.ID, "error", nil, err.Error(), debug)
				continue
			}
			result := map[string]interface{}{
				"containerId": cid,
				"container":   snapshot,
			}
			if err := reportCommand(client, baseURL, token, cmd.ID, "completed", result, "", debug); err != nil {
				return err
			}
		case "rollback-container":
			targetImage := ""
			if v, ok := cmd.Payload["image"].(string); ok {
				targetImage = strings.TrimSpace(v)
			}
			if targetImage == "" {
				_ = reportCommand(client, baseURL, token, cmd.ID, "error", nil, "missing target image", debug)
				continue
			}
			snapshot, err := runRollbackContainer(dockerHost, userAgent, cid, targetImage)
			if err != nil {
				_ = reportCommand(client, baseURL, token, cmd.ID, "error", nil, err.Error(), debug)
				continue
			}
			result := map[string]interface{}{
				"containerId": cid,
				"container":   snapshot,
				"image":       targetImage,
			}
			if err := reportCommand(client, baseURL, token, cmd.ID, "completed", result, "", debug); err != nil {
				return err
			}
		case "fetch-logs":
			tail := 200
			if v, ok := cmd.Payload["tail"].(float64); ok {
				if v > 0 && v <= 2000 {
					tail = int(v)
				}
			}
			logs, err := runFetchLogs(dockerHost, userAgent, cid, tail)
			if err != nil {
				_ = reportCommand(client, baseURL, token, cmd.ID, "error", nil, err.Error(), debug)
				continue
			}
			result := map[string]interface{}{
				"containerId": cid,
				"logs":        logs,
			}
			if err := reportCommand(client, baseURL, token, cmd.ID, "completed", result, "", debug); err != nil {
				return err
			}
		default:
			_ = reportCommand(client, baseURL, token, cmd.ID, "error", nil, "unsupported command type", debug)
		}
	}
}

func fetchNextCommand(client *http.Client, baseURL, token, userAgent string, debug bool) (*agentCommand, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/commands/next", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Agent-Token", token)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("next command failed: %s", resp.Status)
	}

	var cmd agentCommand
	if err := json.NewDecoder(resp.Body).Decode(&cmd); err != nil {
		return nil, err
	}
	if debug {
		fmt.Printf("%s debug: received command %+v\n", time.Now().Format("2006/01/02 - 15:04:05"), cmd)
	}
	return &cmd, nil
}

func reportCommand(client *http.Client, baseURL, token, id, status string, result map[string]interface{}, errMsg string, debug bool) error {
	payload := map[string]interface{}{
		"status": status,
	}
	if result != nil {
		payload["result"] = result
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	if debug {
		fmt.Printf("%s debug: reporting command %s status=%s result=%v error=%s\n", time.Now().Format("2006/01/02 - 15:04:05"), id, status, result, errMsg)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/commands/%s/report", baseURL, id), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("report failed: %s", resp.Status)
	}
	return nil
}

func containerIDFromPayload(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["containerId"].(string); ok {
		return v
	}
	return ""
}

func runCheckUpdate(dockerHost, userAgent, containerID string) (bool, error) {
	cli, err := newDockerClient(dockerHost, userAgent)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	return isUpdateAvailableLocal(ctx, cli, containerID)
}

func runUpdateContainer(dockerHost, userAgent, containerID string) (containerSnapshot, error) {
	cli, err := newDockerClient(dockerHost, userAgent)
	if err != nil {
		return containerSnapshot{}, err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return updateContainerLocal(ctx, cli, containerID)
}

func newDockerClient(dockerHost, userAgent string) (*client.Client, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithUserAgent(userAgent),
		client.WithAPIVersionNegotiation(),
	}
	if strings.TrimSpace(dockerHost) != "" {
		opts = append(opts, client.WithHost(dockerHost))
	}
	return client.NewClientWithOpts(opts...)
}

func isUpdateAvailableLocal(ctx context.Context, cli *client.Client, containerID string) (bool, error) {
	containerInfo, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("inspect container %s: %w", containerID, err)
	}

	localImageInfo, err := cli.ImageInspect(ctx, containerInfo.Image)
	if err != nil {
		return false, fmt.Errorf("inspect local image %s: %w", containerInfo.Image, err)
	}

	dist, err := cli.DistributionInspect(ctx, containerInfo.Config.Image, "")
	if err != nil {
		return false, fmt.Errorf("distribution inspect %s: %w", containerInfo.Config.Image, err)
	}

	remoteDigest := dist.Descriptor.Digest.String()
	for _, localDigest := range localImageInfo.RepoDigests {
		if strings.Contains(localDigest, remoteDigest) {
			return false, nil
		}
	}
	return true, nil
}

func updateContainerLocal(ctx context.Context, cli *client.Client, containerID string) (containerSnapshot, error) {
	containerInfo, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return containerSnapshot{}, fmt.Errorf("inspect container %s: %w", containerID, err)
	}

	out, err := cli.ImagePull(ctx, containerInfo.Config.Image, image.PullOptions{})
	if err != nil {
		return containerSnapshot{}, fmt.Errorf("pull image %s: %w", containerInfo.Config.Image, err)
	}
	defer out.Close()
	// Drain output to avoid blocking
	_, _ = io.Copy(io.Discard, out)

	if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return containerSnapshot{}, fmt.Errorf("stop container %s: %w", containerID, err)
	}

	if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{RemoveVolumes: true, Force: true}); err != nil {
		return containerSnapshot{}, fmt.Errorf("remove container %s: %w", containerID, err)
	}

	networkingConfig := &network.NetworkingConfig{EndpointsConfig: make(map[string]*network.EndpointSettings)}
	for netName, endpoint := range containerInfo.NetworkSettings.Networks {
		networkingConfig.EndpointsConfig[netName] = endpoint
	}

	resp, err := cli.ContainerCreate(ctx, containerInfo.Config, containerInfo.HostConfig, networkingConfig, nil, containerInfo.Name)
	if err != nil {
		return containerSnapshot{}, fmt.Errorf("recreate container %s: %w", containerInfo.Name, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return containerSnapshot{}, fmt.Errorf("start new container %s: %w", resp.ID, err)
	}

	return snapshotContainer(ctx, cli, resp.ID), nil
}

func runRollbackContainer(dockerHost, userAgent, containerID, targetImage string) (containerSnapshot, error) {
	cli, err := newDockerClient(dockerHost, userAgent)
	if err != nil {
		return containerSnapshot{}, err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	containerInfo, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return containerSnapshot{}, fmt.Errorf("inspect container %s: %w", containerID, err)
	}

	out, err := cli.ImagePull(ctx, targetImage, image.PullOptions{})
	if err != nil {
		return containerSnapshot{}, fmt.Errorf("pull image %s: %w", targetImage, err)
	}
	defer out.Close()
	_, _ = io.Copy(io.Discard, out)

	if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return containerSnapshot{}, fmt.Errorf("stop container %s: %w", containerID, err)
	}

	if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{RemoveVolumes: true, Force: true}); err != nil {
		return containerSnapshot{}, fmt.Errorf("remove container %s: %w", containerID, err)
	}

	networkingConfig := &network.NetworkingConfig{EndpointsConfig: make(map[string]*network.EndpointSettings)}
	for netName, endpoint := range containerInfo.NetworkSettings.Networks {
		networkingConfig.EndpointsConfig[netName] = endpoint
	}

	containerInfo.Config.Image = targetImage
	resp, err := cli.ContainerCreate(ctx, containerInfo.Config, containerInfo.HostConfig, networkingConfig, nil, containerInfo.Name)
	if err != nil {
		return containerSnapshot{}, fmt.Errorf("recreate container %s: %w", containerInfo.Name, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return containerSnapshot{}, fmt.Errorf("start new container %s: %w", resp.ID, err)
	}

	return snapshotContainer(ctx, cli, resp.ID), nil
}

func snapshotContainer(ctx context.Context, cli *client.Client, containerID string) containerSnapshot {
	cont, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return containerSnapshot{ID: containerID, State: "unknown", Status: err.Error()}
	}
	name := strings.TrimPrefix(cont.Name, "/")
	var ports []string
	for port, bindings := range cont.NetworkSettings.Ports {
		if len(bindings) == 0 {
			ports = append(ports, port.Port())
			continue
		}
		for _, binding := range bindings {
			if binding.HostPort != "" {
				ports = append(ports, fmt.Sprintf("%s->%s", binding.HostPort, port.Port()))
			} else {
				ports = append(ports, port.Port())
			}
		}
	}
	var labels []string
	for k, v := range cont.Config.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}

	state := cont.State.Status
	status := cont.State.Status
	if cont.State.Running {
		status = cont.State.Status
	}

	return containerSnapshot{
		ID:              containerID,
		Name:            name,
		Image:           cont.Config.Image,
		State:           state,
		Status:          status,
		AutoUpdate:      false,
		UpdateAvailable: false,
		Ports:           ports,
		Labels:          labels,
	}
}

func runFetchLogs(dockerHost, userAgent, containerID string, tail int) (string, error) {
	cli, err := newDockerClient(dockerHost, userAgent)
	if err != nil {
		return "", err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reader, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
	})
	if err != nil {
		return "", fmt.Errorf("fetch logs for %s: %w", containerID, err)
	}
	defer reader.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", fmt.Errorf("read logs for %s: %w", containerID, err)
	}
	combined := strings.TrimSpace(stdout.String() + stderr.String())
	if combined == "" {
		combined = "No logs returned."
	}
	return combined, nil
}
