package containers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/domain"
)

type ContainerService struct {
	db                  *gorm.DB
	dockerClientFactory func() (client.APIClient, error)
}

func NewContainerService(db *gorm.DB) *ContainerService {
	return &ContainerService{
		db: db,
		dockerClientFactory: func() (client.APIClient, error) {
			return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		},
	}
}

func (s *ContainerService) getDockerClient() (client.APIClient, error) {
	return s.dockerClientFactory()
}

type ContainerData struct {
	ID              string
	Name            string
	Image           string
	State           string
	Status          string
	AutoUpdate      bool
	UpdateAvailable bool
	Ports           []string
}

func (s *ContainerService) ListContainers(ctx context.Context) ([]ContainerData, error) {
	cli, err := s.getDockerClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var settings []domain.ContainerSettings
	prefByID := make(map[string]domain.ContainerSettings)
	prefByName := make(map[string]domain.ContainerSettings)
	prefByImage := make(map[string]domain.ContainerSettings)

	if s.db != nil {
		if err := s.db.Find(&settings).Error; err == nil {
			for _, cfg := range settings {
				prefByID[cfg.ID] = cfg
				if cfg.Name != "" {
					prefByName[cfg.Name] = cfg
				}
				if cfg.Image != "" {
					prefByImage[cfg.Image] = cfg
				}
			}
		}
	}

	result := make([]ContainerData, 0, len(containers))
	for _, cont := range containers {
		name := ""
		if len(cont.Names) > 0 {
			name = strings.TrimPrefix(cont.Names[0], "/")
		}

		pref := prefByID[cont.ID]
		if pref.ID == "" && name != "" {
			pref = prefByName[name]
		}
		if pref.ID == "" && cont.Image != "" {
			pref = prefByImage[cont.Image]
		}

		// Sync ID if it changed (recreation) but we found match by name/image
		if pref.ID != "" && pref.ID != cont.ID && s.db != nil {
			pref.ID = cont.ID
			pref.Name = name
			pref.Image = cont.Image
			_ = s.db.Session(&gorm.Session{Logger: logger.Discard}).Save(&pref).Error
		}

		var ports []string
		for _, p := range cont.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}

		result = append(result, ContainerData{
			ID:              cont.ID,
			Name:            name,
			Image:           cont.Image,
			State:           cont.State,
			Status:          cont.Status,
			AutoUpdate:      pref.AutoUpdate,
			UpdateAvailable: pref.UpdateAvailable,
			Ports:           ports,
		})

		// Ensure name is set in DB if missing
		if pref.Name == "" && name != "" && s.db != nil {
			_ = s.db.Model(&domain.ContainerSettings{}).Where("id = ?", cont.ID).Update("name", name)
		}
	}

	return result, nil
}

func (s *ContainerService) GetHostInfo(ctx context.Context) (map[string]interface{}, error) {
	cli, err := s.getDockerClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	ping, _ := cli.Ping(ctx)
	info, _ := cli.Info(ctx)

	dockerVersion := info.ServerVersion
	if dockerVersion == "" {
		dockerVersion = ping.APIVersion
	}
	if dockerVersion == "" {
		dockerVersion = "unknown"
	}

	hostname := info.Name
	if hostname == "" {
		hostname = "localhost"
	}

	platform := fmt.Sprintf("%s/%s", info.OSType, info.Architecture)
	platform = strings.Trim(platform, "/")
	if platform == "" {
		platform = "unknown"
	}

	result := map[string]interface{}{
		"dockerVersion": dockerVersion,
		"platform":      platform,
		"hostname":      hostname,
		"lastSeen":      time.Now().Format(time.RFC3339),
	}

	if c, err := cpu.Percent(0, false); err == nil && len(c) > 0 {
		result["cpu"] = c[0]
	}
	if m, err := mem.VirtualMemory(); err == nil {
		result["memory"] = m.UsedPercent
	}

	return result, nil
}

func (s *ContainerService) CheckUpdate(ctx context.Context, containerID string) (bool, error) {
	cli, err := s.getDockerClient()
	if err != nil {
		return false, err
	}
	defer cli.Close()

	available, err := IsUpdateAvailableWithClient(ctx, cli, containerID)
	if err != nil {
		return false, err
	}

	if s.db != nil {
		silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
		update := map[string]interface{}{"update_available": available}
		err := silentDB.Model(&domain.ContainerSettings{}).Where("id = ?", containerID).Updates(update).Error
		if err != nil || silentDB.RowsAffected == 0 {
			// Discovery fallback
			if info, inspectErr := cli.ContainerInspect(ctx, containerID); inspectErr == nil {
				name := strings.TrimPrefix(info.Name, "/")
				update["name"] = name
				update["image"] = info.Config.Image
				_ = silentDB.Where("name = ? OR image = ?", name, info.Config.Image).Assign(update).FirstOrCreate(&domain.ContainerSettings{ID: containerID, Name: name, Image: info.Config.Image}).Error
			}
		}
	}

	return available, nil
}

func (s *ContainerService) ToggleAutoUpdate(ctx context.Context, id string, enabled bool) error {
	name := ""
	imageRef := ""

	cli, err := s.getDockerClient()
	if err == nil {
		defer cli.Close()
		if info, inspectErr := cli.ContainerInspect(ctx, id); inspectErr == nil {
			name = strings.TrimPrefix(info.Name, "/")
			imageRef = info.Config.Image
		}
	}

	if s.db == nil {
		return fmt.Errorf("database not available")
	}

	var cfg domain.ContainerSettings
	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
	err = silentDB.Where("id = ?", id).First(&cfg).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Try finding by name or image if ID not found
		if name != "" {
			_ = silentDB.Where("name = ?", name).First(&cfg).Error
		}
		if cfg.ID == "" && imageRef != "" {
			_ = silentDB.Where("image = ?", imageRef).First(&cfg).Error
		}

		if cfg.ID == "" {
			// Create new
			cfg = domain.ContainerSettings{
				ID:         id,
				Name:       name,
				Image:      imageRef,
				AutoUpdate: enabled,
			}
			return silentDB.Create(&cfg).Error
		}
	}

	// Update existing
	cfg.ID = id
	cfg.AutoUpdate = enabled
	if name != "" {
		cfg.Name = name
	}
	if imageRef != "" {
		cfg.Image = imageRef
	}
	return silentDB.Save(&cfg).Error
}

func (s *ContainerService) StartContainer(ctx context.Context, id string) error {
	cli, err := s.getDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()
	return cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (s *ContainerService) StopContainer(ctx context.Context, id string) error {
	cli, err := s.getDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()
	timeout := 30
	return cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (s *ContainerService) RestartContainer(ctx context.Context, id string) error {
	cli, err := s.getDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()
	timeout := 30
	return cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (s *ContainerService) GetLogs(ctx context.Context, id string, tail string) (string, error) {
	cli, err := s.getDockerClient()
	if err != nil {
		return "", err
	}
	defer cli.Close()

	reader, err := cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Tail:       tail,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, reader)
	return buf.String(), err
}

func (s *ContainerService) CountAutoUpdate() (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("database not available")
	}
	var count int64
	err := s.db.Model(&domain.ContainerSettings{}).Where("auto_update = ?", true).Count(&count).Error
	return count, err
}

// UpdateProgressCallback is used to stream status updates
type UpdateProgressCallback func(map[string]interface{})

// UpdateError describes an update failure; RolledBack indicates the old container was restored.
type UpdateError struct {
	Err             error
	RolledBack      bool
	RollbackMessage string
}

func (e *UpdateError) Error() string {
	if e == nil {
		return ""
	}
	if e.RolledBack && e.RollbackMessage != "" {
		return fmt.Sprintf("%s (rolled back: %s)", e.Err, e.RollbackMessage)
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "update failed"
}

func (e *UpdateError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (s *ContainerService) UpdateContainer(ctx context.Context, id string, progress UpdateProgressCallback) (string, string, string, string, error) {
	cli, err := s.getDockerClient()
	if err != nil {
		return "", "", "", "", err
	}
	defer cli.Close()

	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to inspect container: %w", err)
	}

	sendProgress := func(status string) {
		if progress != nil {
			progress(map[string]interface{}{"status": status})
		}
	}

	sendProgress("Pulling image " + info.Config.Image)

	targetRef := info.Config.Image
	out, err := cli.ImagePull(ctx, targetRef, image.PullOptions{})
	if err != nil {
		return "", "", "", "", fmt.Errorf("image pull failed: %w", err)
	}
	defer out.Close()

	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		line := scanner.Text()
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err == nil {
			if progress != nil {
				progress(parsed)
			}
		} else if progress != nil {
			progress(map[string]interface{}{"status": line})
		}
	}

	digest := resolveImageDigest(ctx, cli, targetRef)

	sendProgress("Stopping container")
	if err := cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return "", "", "", "", fmt.Errorf("failed to stop container: %w", err)
	}

	name := strings.TrimPrefix(info.Name, "/")
	backupName := fmt.Sprintf("%s-updockly-backup-%d", name, time.Now().Unix())
	sendProgress("Backing up container before recreate")
	if err := cli.ContainerRename(ctx, id, backupName); err != nil {
		_ = cli.ContainerStart(ctx, id, container.StartOptions{})
		return "", "", "", "", fmt.Errorf("failed to backup container: %w", err)
	}
	backupID := id

	restoreOriginal := func(reason error) (string, string, string, string, error) {
		sendProgress("Rolling back to previous container")
		if err := cli.ContainerRename(ctx, backupID, name); err != nil {
			// Best effort start even if rename fails to avoid downtime
			_ = cli.ContainerStart(ctx, backupID, container.StartOptions{})
			return backupID, name, info.Config.Image, "", &UpdateError{
				Err: fmt.Errorf("failed to recreate container (%v) and could not restore original name: %w", reason, err),
			}
		}
		if err := cli.ContainerStart(ctx, backupID, container.StartOptions{}); err != nil {
			return backupID, name, info.Config.Image, "", &UpdateError{
				Err: fmt.Errorf("failed to recreate container (%v) and restart the original one: %w", reason, err),
			}
		}
		return backupID, name, info.Config.Image, "", &UpdateError{
			Err:             reason,
			RolledBack:      true,
			RollbackMessage: "restored previous container",
		}
	}

	sendProgress("Recreating container")
	networkingConfig := &network.NetworkingConfig{EndpointsConfig: make(map[string]*network.EndpointSettings)}
	for netName, endpoint := range info.NetworkSettings.Networks {
		networkingConfig.EndpointsConfig[netName] = endpoint
	}

	configCopy := *info.Config
	hostConfigCopy := *info.HostConfig
	if hostConfigCopy.NetworkMode.IsHost() {
		// Docker does not allow setting hostname with host network mode; clear to avoid recreate failure.
		configCopy.Hostname = ""
		configCopy.Domainname = ""
	}

	resp, err := cli.ContainerCreate(ctx, &configCopy, &hostConfigCopy, networkingConfig, nil, name)
	if err != nil {
		return restoreOriginal(fmt.Errorf("failed to recreate container: %w", err))
	}

	sendProgress("Starting new container")
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{RemoveVolumes: true, Force: true})
		return restoreOriginal(fmt.Errorf("failed to start container: %w", err))
	}

	sendProgress("Cleaning up old container")
	_ = cli.ContainerRemove(ctx, backupID, container.RemoveOptions{RemoveVolumes: true, Force: true})

	// Sync DB
	newDigest := digest
	if s.db != nil {
		silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
		_ = silentDB.Model(&domain.ContainerSettings{}).Where("id = ?", id).Updates(map[string]interface{}{
			"id":               resp.ID,
			"update_available": false,
			"name":             name,
			"image":            info.Config.Image,
		}).Error
	}

	return resp.ID, name, info.Config.Image, newDigest, nil
}

func (s *ContainerService) RollbackContainer(ctx context.Context, id, targetImage string) (string, string, error) {
	cli, err := s.getDockerClient()
	if err != nil {
		return "", "", err
	}
	defer cli.Close()

	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", "", fmt.Errorf("failed to inspect container: %w", err)
	}
	name := strings.TrimPrefix(info.Name, "/")

	pullResp, err := cli.ImagePull(ctx, targetImage, image.PullOptions{})
	if err != nil {
		return name, "", fmt.Errorf("failed to pull image: %w", err)
	}
	defer pullResp.Close()
	_, _ = io.Copy(io.Discard, pullResp)

	if err := cli.ContainerStop(ctx, info.ID, container.StopOptions{}); err != nil {
		// Log but continue? Original code logged but continued.
	}

	if err := cli.ContainerRemove(ctx, info.ID, container.RemoveOptions{RemoveVolumes: true, Force: true}); err != nil {
		return name, "", fmt.Errorf("failed to remove container: %w", err)
	}

	networkingConfig := &network.NetworkingConfig{EndpointsConfig: make(map[string]*network.EndpointSettings)}
	for netName, endpoint := range info.NetworkSettings.Networks {
		networkingConfig.EndpointsConfig[netName] = endpoint
	}

	info.Config.Image = targetImage
	resp, err := cli.ContainerCreate(ctx, info.Config, info.HostConfig, networkingConfig, nil, name)
	if err != nil {
		return name, "", fmt.Errorf("failed to recreate container: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return name, "", fmt.Errorf("failed to start container: %w", err)
	}

	if s.db != nil {
		silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
		_ = silentDB.Model(&domain.ContainerSettings{}).Where("id = ?", info.ID).Updates(map[string]interface{}{
			"id":               resp.ID,
			"name":             name,
			"image":            targetImage,
			"update_available": false,
		}).Error
	}

	return name, resp.ID, nil
}

// Helper function moved from updater.go (or duplicated/adapted)
func IsUpdateAvailableWithClient(ctx context.Context, cli client.APIClient, containerID string) (bool, error) {
	containerInfo, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	localImageInfo, err := cli.ImageInspect(ctx, containerInfo.Image)
	if err != nil {
		return false, fmt.Errorf("failed to inspect local image %s: %w", containerInfo.Image, err)
	}

	dist, err := cli.DistributionInspect(ctx, containerInfo.Config.Image, "")
	if err != nil {
		return false, fmt.Errorf("failed to inspect image distribution %s: %w", containerInfo.Config.Image, err)
	}

	remoteDigest := dist.Descriptor.Digest

	for _, localDigest := range localImageInfo.RepoDigests {
		if strings.Contains(localDigest, remoteDigest.String()) {
			return false, nil
		}
	}

	return true, nil
}

func resolveImageDigest(ctx context.Context, cli client.APIClient, ref string) string {
	if strings.TrimSpace(ref) == "" {
		return ""
	}
	inspect, _, err := cli.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return ""
	}
	if len(inspect.RepoDigests) > 0 && inspect.RepoDigests[0] != "" {
		return inspect.RepoDigests[0]
	}
	return inspect.ID
}
