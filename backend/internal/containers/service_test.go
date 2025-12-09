package containers

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockDockerClient embeds client.APIClient to satisfy the interface
// and allow selective overriding.
type MockDockerClient struct {
	client.APIClient

	// Mock functions
	ContainerListFunc       func(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerInspectFunc    func(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ImageInspectFunc        func(ctx context.Context, imageID string) (image.InspectResponse, error)
	DistributionInspectFunc func(ctx context.Context, imageRef, encodedRegistryAuth string) (registry.DistributionInspect, error)
	ContainerStartFunc      func(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStopFunc       func(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRestartFunc    func(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerLogsFunc       func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ImagePullFunc           func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerRemoveFunc     func(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerCreateFunc     func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	ImageInspectWithRawFunc func(ctx context.Context, imageID string) (image.InspectResponse, []byte, error)
	PingFunc                func(ctx context.Context) (types.Ping, error)
	InfoFunc                func(ctx context.Context) (system.Info, error)
}

func (m *MockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	if m.ContainerListFunc != nil {
		return m.ContainerListFunc(ctx, options)
	}
	return nil, nil
}

func (m *MockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	if m.ContainerInspectFunc != nil {
		return m.ContainerInspectFunc(ctx, containerID)
	}
	return types.ContainerJSON{}, nil
}

// Correct signature matching the error message
func (m *MockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	if m.ImageInspectWithRawFunc != nil {
		return m.ImageInspectWithRawFunc(ctx, imageID)
	}
	return image.InspectResponse{}, nil, nil
}

// Implementing the one requested by error message
func (m *MockDockerClient) ImageInspect(ctx context.Context, imageID string, opts ...client.ImageInspectOption) (image.InspectResponse, error) {
	if m.ImageInspectFunc != nil {
		return m.ImageInspectFunc(ctx, imageID)
	}
	return image.InspectResponse{}, nil
}

func (m *MockDockerClient) DistributionInspect(ctx context.Context, imageRef, encodedRegistryAuth string) (registry.DistributionInspect, error) {
	if m.DistributionInspectFunc != nil {
		return m.DistributionInspectFunc(ctx, imageRef, encodedRegistryAuth)
	}
	return registry.DistributionInspect{}, nil
}

func (m *MockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.ContainerStartFunc != nil {
		return m.ContainerStartFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.ContainerStopFunc != nil {
		return m.ContainerStopFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockDockerClient) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.ContainerRestartFunc != nil {
		return m.ContainerRestartFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockDockerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	if m.ContainerLogsFunc != nil {
		return m.ContainerLogsFunc(ctx, containerID, options)
	}
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

func (m *MockDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	if m.ImagePullFunc != nil {
		return m.ImagePullFunc(ctx, ref, options)
	}
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

func (m *MockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.ContainerRemoveFunc != nil {
		return m.ContainerRemoveFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	if m.ContainerCreateFunc != nil {
		return m.ContainerCreateFunc(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{}, nil
}

func (m *MockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return types.Ping{}, nil
}

func (m *MockDockerClient) Info(ctx context.Context) (system.Info, error) {
	if m.InfoFunc != nil {
		return m.InfoFunc(ctx)
	}
	return system.Info{}, nil
}

func (m *MockDockerClient) Close() error {
	return nil
}

// Test setup helper
func setupContainerServiceTest(t *testing.T) (*ContainerService, *MockDockerClient, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ContainerSettings{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mockClient := &MockDockerClient{}
	svc := NewContainerService(db)
	svc.dockerClientFactory = func() (client.APIClient, error) {
		return mockClient, nil
	}

	return svc, mockClient, db
}

func TestListContainers(t *testing.T) {
	svc, mock, _ := setupContainerServiceTest(t)

	mock.ContainerListFunc = func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
		return []types.Container{
			{
				ID:      "123",
				Names:   []string{"/test-container"},
				Image:   "test-image",
				State:   "running",
				Status:  "Up 2 hours",
				Created: time.Now().Unix(),
			},
		}, nil
	}

	list, err := svc.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 container, got %d", len(list))
	}
	if list[0].Name != "test-container" {
		t.Errorf("expected name test-container, got %s", list[0].Name)
	}
}

func TestToggleAutoUpdate(t *testing.T) {
	svc, mock, db := setupContainerServiceTest(t)

	mock.ContainerInspectFunc = func(ctx context.Context, containerID string) (types.ContainerJSON, error) {
		return types.ContainerJSON{
			Config: &container.Config{
				Image: "test-image:latest",
			},
			ContainerJSONBase: &types.ContainerJSONBase{
				Name: "/test-container",
			},
		}, nil
	}

	err := svc.ToggleAutoUpdate(context.Background(), "123", true)
	if err != nil {
		t.Fatalf("ToggleAutoUpdate failed: %v", err)
	}

	var cfg ContainerSettings
	if err := db.First(&cfg, "id = ?", "123").Error; err != nil {
		t.Fatalf("settings not saved: %v", err)
	}
	if !cfg.AutoUpdate {
		t.Error("AutoUpdate not set to true")
	}
	if cfg.Name != "test-container" {
		t.Errorf("expected name test-container, got %s", cfg.Name)
	}
}

func TestStartStopRestartContainer(t *testing.T) {
	svc, mock, _ := setupContainerServiceTest(t)

	startCalled := false
	mock.ContainerStartFunc = func(ctx context.Context, containerID string, options container.StartOptions) error {
		startCalled = true
		return nil
	}
	if err := svc.StartContainer(context.Background(), "123"); err != nil {
		t.Fatal(err)
	}
	if !startCalled {
		t.Error("ContainerStart was not called")
	}

	stopCalled := false
	mock.ContainerStopFunc = func(ctx context.Context, containerID string, options container.StopOptions) error {
		stopCalled = true
		return nil
	}
	if err := svc.StopContainer(context.Background(), "123"); err != nil {
		t.Fatal(err)
	}
	if !stopCalled {
		t.Error("ContainerStop was not called")
	}

	restartCalled := false
	mock.ContainerRestartFunc = func(ctx context.Context, containerID string, options container.StopOptions) error {
		restartCalled = true
		return nil
	}
	if err := svc.RestartContainer(context.Background(), "123"); err != nil {
		t.Fatal(err)
	}
	if !restartCalled {
		t.Error("ContainerRestart was not called")
	}
}

func TestGetLogs(t *testing.T) {
	svc, mock, _ := setupContainerServiceTest(t)

	mock.ContainerLogsFunc = func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte("log output"))), nil
	}

	logs, err := svc.GetLogs(context.Background(), "123", "100")
	if err != nil {
		t.Fatal(err)
	}
	if logs != "log output" {
		t.Errorf("expected 'log output', got %q", logs)
	}
}
