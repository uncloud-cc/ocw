// Package mock provides a mock container runtime for testing.
package mock

import (
	"context"
	"io"
	"time"

	"github.com/uncloud-cc/ocw/pkg/container"
)

// Runtime is a mock implementation of container.Runtime for testing.
type Runtime struct {
	// Image operations
	PullFunc        func(ctx context.Context, image string, opts container.PullOptions) error
	BuildFunc       func(ctx context.Context, opts container.BuildOptions) (container.ImageID, error)
	ImageExistsFunc func(ctx context.Context, image string) (bool, error)

	// Container lifecycle
	CreateFunc func(ctx context.Context, opts container.CreateOptions) (container.ContainerID, error)
	StartFunc  func(ctx context.Context, id container.ContainerID) error
	StopFunc   func(ctx context.Context, id container.ContainerID, timeout time.Duration) error
	RemoveFunc func(ctx context.Context, id container.ContainerID, force bool) error
	KillFunc   func(ctx context.Context, id container.ContainerID, signal string) error

	// Container inspection
	WaitFunc    func(ctx context.Context, id container.ContainerID) (container.ExitResult, error)
	InspectFunc func(ctx context.Context, id container.ContainerID) (container.ContainerInfo, error)

	// Logs and I/O
	LogsFunc   func(ctx context.Context, id container.ContainerID, opts container.LogOptions) (io.ReadCloser, error)
	AttachFunc func(ctx context.Context, id container.ContainerID, opts container.AttachOptions) (container.Streams, error)

	// Exec
	ExecFunc func(ctx context.Context, id container.ContainerID, cmd []string) (container.ExitResult, error)

	// Volumes
	CreateVolumeFunc func(ctx context.Context, name string) (container.VolumeID, error)
	RemoveVolumeFunc func(ctx context.Context, id container.VolumeID) error

	// Networks
	CreateNetworkFunc  func(ctx context.Context, name string, opts container.NetworkOptions) (container.NetworkID, error)
	RemoveNetworkFunc  func(ctx context.Context, id container.NetworkID) error
	ConnectNetworkFunc func(ctx context.Context, networkID container.NetworkID, containerID container.ContainerID) error

	// Call tracking for assertions
	PullCalls           []PullCall
	BuildCalls          []BuildCall
	ImageExistsCalls    []ImageExistsCall
	CreateCalls         []CreateCall
	StartCalls          []StartCall
	StopCalls           []StopCall
	RemoveCalls         []RemoveCall
	KillCalls           []KillCall
	WaitCalls           []WaitCall
	InspectCalls        []InspectCall
	LogsCalls           []LogsCall
	AttachCalls         []AttachCall
	ExecCalls           []ExecCall
	CreateVolumeCalls   []CreateVolumeCall
	RemoveVolumeCalls   []RemoveVolumeCall
	CreateNetworkCalls  []CreateNetworkCall
	RemoveNetworkCalls  []RemoveNetworkCall
	ConnectNetworkCalls []ConnectNetworkCall
}

// Call tracking types
type PullCall struct {
	Image string
	Opts  container.PullOptions
}

type BuildCall struct {
	Opts container.BuildOptions
}

type ImageExistsCall struct {
	Image string
}

type CreateCall struct {
	Opts container.CreateOptions
}

type StartCall struct {
	ID container.ContainerID
}

type StopCall struct {
	ID      container.ContainerID
	Timeout time.Duration
}

type RemoveCall struct {
	ID    container.ContainerID
	Force bool
}

type KillCall struct {
	ID     container.ContainerID
	Signal string
}

type WaitCall struct {
	ID container.ContainerID
}

type InspectCall struct {
	ID container.ContainerID
}

type LogsCall struct {
	ID   container.ContainerID
	Opts container.LogOptions
}

type AttachCall struct {
	ID   container.ContainerID
	Opts container.AttachOptions
}

type ExecCall struct {
	ID  container.ContainerID
	Cmd []string
}

type CreateVolumeCall struct {
	Name string
}

type RemoveVolumeCall struct {
	ID container.VolumeID
}

type CreateNetworkCall struct {
	Name string
	Opts container.NetworkOptions
}

type RemoveNetworkCall struct {
	ID container.NetworkID
}

type ConnectNetworkCall struct {
	NetworkID   container.NetworkID
	ContainerID container.ContainerID
}

// Verify Runtime implements container.Runtime
var _ container.Runtime = (*Runtime)(nil)

// Pull implements container.Runtime.
func (m *Runtime) Pull(ctx context.Context, image string, opts container.PullOptions) error {
	m.PullCalls = append(m.PullCalls, PullCall{Image: image, Opts: opts})
	if m.PullFunc != nil {
		return m.PullFunc(ctx, image, opts)
	}
	return nil
}

// Build implements container.Runtime.
func (m *Runtime) Build(ctx context.Context, opts container.BuildOptions) (container.ImageID, error) {
	m.BuildCalls = append(m.BuildCalls, BuildCall{Opts: opts})
	if m.BuildFunc != nil {
		return m.BuildFunc(ctx, opts)
	}
	return "mock-image-id", nil
}

// ImageExists implements container.Runtime.
func (m *Runtime) ImageExists(ctx context.Context, image string) (bool, error) {
	m.ImageExistsCalls = append(m.ImageExistsCalls, ImageExistsCall{Image: image})
	if m.ImageExistsFunc != nil {
		return m.ImageExistsFunc(ctx, image)
	}
	return true, nil
}

// Create implements container.Runtime.
func (m *Runtime) Create(ctx context.Context, opts container.CreateOptions) (container.ContainerID, error) {
	m.CreateCalls = append(m.CreateCalls, CreateCall{Opts: opts})
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, opts)
	}
	return "mock-container-id", nil
}

// Start implements container.Runtime.
func (m *Runtime) Start(ctx context.Context, id container.ContainerID) error {
	m.StartCalls = append(m.StartCalls, StartCall{ID: id})
	if m.StartFunc != nil {
		return m.StartFunc(ctx, id)
	}
	return nil
}

// Stop implements container.Runtime.
func (m *Runtime) Stop(ctx context.Context, id container.ContainerID, timeout time.Duration) error {
	m.StopCalls = append(m.StopCalls, StopCall{ID: id, Timeout: timeout})
	if m.StopFunc != nil {
		return m.StopFunc(ctx, id, timeout)
	}
	return nil
}

// Remove implements container.Runtime.
func (m *Runtime) Remove(ctx context.Context, id container.ContainerID, force bool) error {
	m.RemoveCalls = append(m.RemoveCalls, RemoveCall{ID: id, Force: force})
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, id, force)
	}
	return nil
}

// Kill implements container.Runtime.
func (m *Runtime) Kill(ctx context.Context, id container.ContainerID, signal string) error {
	m.KillCalls = append(m.KillCalls, KillCall{ID: id, Signal: signal})
	if m.KillFunc != nil {
		return m.KillFunc(ctx, id, signal)
	}
	return nil
}

// Wait implements container.Runtime.
func (m *Runtime) Wait(ctx context.Context, id container.ContainerID) (container.ExitResult, error) {
	m.WaitCalls = append(m.WaitCalls, WaitCall{ID: id})
	if m.WaitFunc != nil {
		return m.WaitFunc(ctx, id)
	}
	return container.ExitResult{StatusCode: 0}, nil
}

// Inspect implements container.Runtime.
func (m *Runtime) Inspect(ctx context.Context, id container.ContainerID) (container.ContainerInfo, error) {
	m.InspectCalls = append(m.InspectCalls, InspectCall{ID: id})
	if m.InspectFunc != nil {
		return m.InspectFunc(ctx, id)
	}
	return container.ContainerInfo{ID: id}, nil
}

// Logs implements container.Runtime.
func (m *Runtime) Logs(ctx context.Context, id container.ContainerID, opts container.LogOptions) (io.ReadCloser, error) {
	m.LogsCalls = append(m.LogsCalls, LogsCall{ID: id, Opts: opts})
	if m.LogsFunc != nil {
		return m.LogsFunc(ctx, id, opts)
	}
	return nil, nil
}

// Attach implements container.Runtime.
func (m *Runtime) Attach(ctx context.Context, id container.ContainerID, opts container.AttachOptions) (container.Streams, error) {
	m.AttachCalls = append(m.AttachCalls, AttachCall{ID: id, Opts: opts})
	if m.AttachFunc != nil {
		return m.AttachFunc(ctx, id, opts)
	}
	return container.Streams{}, nil
}

// Exec implements container.Runtime.
func (m *Runtime) Exec(ctx context.Context, id container.ContainerID, cmd []string) (container.ExitResult, error) {
	m.ExecCalls = append(m.ExecCalls, ExecCall{ID: id, Cmd: cmd})
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, id, cmd)
	}
	return container.ExitResult{StatusCode: 0}, nil
}

// CreateVolume implements container.Runtime.
func (m *Runtime) CreateVolume(ctx context.Context, name string) (container.VolumeID, error) {
	m.CreateVolumeCalls = append(m.CreateVolumeCalls, CreateVolumeCall{Name: name})
	if m.CreateVolumeFunc != nil {
		return m.CreateVolumeFunc(ctx, name)
	}
	return container.VolumeID(name), nil
}

// RemoveVolume implements container.Runtime.
func (m *Runtime) RemoveVolume(ctx context.Context, id container.VolumeID) error {
	m.RemoveVolumeCalls = append(m.RemoveVolumeCalls, RemoveVolumeCall{ID: id})
	if m.RemoveVolumeFunc != nil {
		return m.RemoveVolumeFunc(ctx, id)
	}
	return nil
}

// CreateNetwork implements container.Runtime.
func (m *Runtime) CreateNetwork(ctx context.Context, name string, opts container.NetworkOptions) (container.NetworkID, error) {
	m.CreateNetworkCalls = append(m.CreateNetworkCalls, CreateNetworkCall{Name: name, Opts: opts})
	if m.CreateNetworkFunc != nil {
		return m.CreateNetworkFunc(ctx, name, opts)
	}
	return container.NetworkID(name), nil
}

// RemoveNetwork implements container.Runtime.
func (m *Runtime) RemoveNetwork(ctx context.Context, id container.NetworkID) error {
	m.RemoveNetworkCalls = append(m.RemoveNetworkCalls, RemoveNetworkCall{ID: id})
	if m.RemoveNetworkFunc != nil {
		return m.RemoveNetworkFunc(ctx, id)
	}
	return nil
}

// ConnectNetwork implements container.Runtime.
func (m *Runtime) ConnectNetwork(ctx context.Context, networkID container.NetworkID, containerID container.ContainerID) error {
	m.ConnectNetworkCalls = append(m.ConnectNetworkCalls, ConnectNetworkCall{NetworkID: networkID, ContainerID: containerID})
	if m.ConnectNetworkFunc != nil {
		return m.ConnectNetworkFunc(ctx, networkID, containerID)
	}
	return nil
}
