package server

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
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ContainerService struct {
	db *gorm.DB
}

func NewContainerService(db *gorm.DB) *ContainerService {
	return &ContainerService{db: db}
}

func (s *ContainerService) getDockerClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

type ContainerData struct {
	ID              string
	Name            string
	Image           string
	State           string
	Status          string
	AutoUpdate      bool
	UpdateAvailable bool
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

	var settings []ContainerSettings
	prefByID := make(map[string]ContainerSettings)
	prefByName := make(map[string]ContainerSettings)
	prefByImage := make(map[string]ContainerSettings)

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

		result = append(result, ContainerData{
			ID:              cont.ID,
			Name:            name,
			Image:           cont.Image,
			State:           cont.State,
			Status:          cont.Status,
			AutoUpdate:      pref.AutoUpdate,
			UpdateAvailable: pref.UpdateAvailable,
		})

		// Ensure name is set in DB if missing
		if pref.Name == "" && name != "" && s.db != nil {
			_ = s.db.Model(&ContainerSettings{}).Where("id = ?", cont.ID).Update("name", name)
		}
	}

	return result, nil
}

func (s *ContainerService) GetHostInfo(ctx context.Context) (map[string]string, error) {
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

	return map[string]string{
		"dockerVersion": dockerVersion,
		"platform":      platform,
		"hostname":      hostname,
		"lastSeen":      time.Now().Format(time.RFC3339),
	}, nil
}

func (s *ContainerService) CheckUpdate(ctx context.Context, containerID string) (bool, error) {
	cli, err := s.getDockerClient()
	if err != nil {
		return false, err
	}
	defer cli.Close()

	available, err := isUpdateAvailableWithClient(ctx, cli, containerID)
	if err != nil {
		return false, err
	}

	if s.db != nil {
		silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
		update := map[string]interface{}{"update_available": available}
		err := silentDB.Model(&ContainerSettings{}).Where("id = ?", containerID).Updates(update).Error
		if err != nil || silentDB.RowsAffected == 0 {
			// Discovery fallback
			if info, inspectErr := cli.ContainerInspect(ctx, containerID); inspectErr == nil {
				name := strings.TrimPrefix(info.Name, "/")
				update["name"] = name
				update["image"] = info.Config.Image
				_ = silentDB.Where("name = ? OR image = ?", name, info.Config.Image).Assign(update).FirstOrCreate(&ContainerSettings{ID: containerID, Name: name, Image: info.Config.Image}).Error
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

	var cfg ContainerSettings
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
			cfg = ContainerSettings{
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
	err := s.db.Model(&ContainerSettings{}).Where("auto_update = ?", true).Count(&count).Error
	return count, err
}

// UpdateProgressCallback is used to stream status updates
type UpdateProgressCallback func(map[string]interface{})

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

	progress(map[string]interface{}{"status": "Pulling image " + info.Config.Image})

	out, err := cli.ImagePull(ctx, info.Config.Image, image.PullOptions{})
	if err != nil {
		return "", "", "", "", fmt.Errorf("image pull failed: %w", err)
	}
	defer out.Close()

	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		line := scanner.Text()
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err == nil {
			progress(parsed)
		} else {
			progress(map[string]interface{}{"status": line})
		}
	}

	progress(map[string]interface{}{"status": "Stopping container"})
	if err := cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return "", "", "", "", fmt.Errorf("failed to stop container: %w", err)
	}

	progress(map[string]interface{}{"status": "Removing container"})
	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{RemoveVolumes: true, Force: true}); err != nil {
		return "", "", "", "", fmt.Errorf("failed to remove container: %w", err)
	}

	progress(map[string]interface{}{"status": "Recreating container"})
	networkingConfig := &network.NetworkingConfig{EndpointsConfig: make(map[string]*network.EndpointSettings)}
	for netName, endpoint := range info.NetworkSettings.Networks {
		networkingConfig.EndpointsConfig[netName] = endpoint
	}

	name := strings.TrimPrefix(info.Name, "/")
	resp, err := cli.ContainerCreate(ctx, info.Config, info.HostConfig, networkingConfig, nil, name)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to recreate container: %w", err)
	}

	progress(map[string]interface{}{"status": "Starting new container"})
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", "", "", "", fmt.Errorf("failed to start container: %w", err)
	}

	// Sync DB
	newDigest := resolveImageDigest(ctx, cli, info.Config.Image)
	if s.db != nil {
		silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
		_ = silentDB.Model(&ContainerSettings{}).Where("id = ?", id).Updates(map[string]interface{}{
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
		_ = silentDB.Model(&ContainerSettings{}).Where("id = ?", info.ID).Updates(map[string]interface{}{
			"id":               resp.ID,
			"name":             name,
			"image":            targetImage,
			"update_available": false,
		}).Error
	}

	return name, resp.ID, nil
}

// Helper function moved from updater.go (or duplicated/adapted)
func isUpdateAvailableWithClient(ctx context.Context, cli *client.Client, containerID string) (bool, error) {
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

func resolveImageDigest(ctx context.Context, cli *client.Client, ref string) string {
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
