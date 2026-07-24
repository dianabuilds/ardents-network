package docker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

type hungDockerTransport struct {
	entered   chan struct{}
	cancelled chan struct{}
	once      sync.Once
	hangStop  bool
}

func newHungDockerTransport() *hungDockerTransport {
	return &hungDockerTransport{entered: make(chan struct{}), cancelled: make(chan struct{})}
}

func (t *hungDockerTransport) ContainerList(ctx context.Context, _ client.ContainerListOptions) (client.ContainerListResult, error) {
	t.once.Do(func() { close(t.entered) })
	<-ctx.Done()
	close(t.cancelled)
	return client.ContainerListResult{}, ctx.Err()
}

func (*hungDockerTransport) ContainerCreate(context.Context, client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	return client.ContainerCreateResult{}, errors.New("unexpected ContainerCreate")
}
func (*hungDockerTransport) ContainerStart(context.Context, string, client.ContainerStartOptions) (client.ContainerStartResult, error) {
	return client.ContainerStartResult{}, errors.New("unexpected ContainerStart")
}
func (*hungDockerTransport) ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{}, errors.New("unexpected ContainerInspect")
}
func (*hungDockerTransport) ContainerRemove(context.Context, string, client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	return client.ContainerRemoveResult{}, errors.New("unexpected ContainerRemove")
}
func (t *hungDockerTransport) ContainerStop(ctx context.Context, _ string, _ client.ContainerStopOptions) (client.ContainerStopResult, error) {
	if t.hangStop {
		t.once.Do(func() { close(t.entered) })
		<-ctx.Done()
		close(t.cancelled)
		return client.ContainerStopResult{}, ctx.Err()
	}
	return client.ContainerStopResult{}, errors.New("unexpected ContainerStop")
}
func (*hungDockerTransport) ImageInspect(context.Context, string, ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	return client.ImageInspectResult{}, errors.New("unexpected ImageInspect")
}
func (*hungDockerTransport) NetworkInspect(context.Context, string, client.NetworkInspectOptions) (client.NetworkInspectResult, error) {
	return client.NetworkInspectResult{}, errors.New("unexpected NetworkInspect")
}
func (*hungDockerTransport) NetworkCreate(context.Context, string, client.NetworkCreateOptions) (client.NetworkCreateResult, error) {
	return client.NetworkCreateResult{}, errors.New("unexpected NetworkCreate")
}
func (*hungDockerTransport) NetworkRemove(context.Context, string, client.NetworkRemoveOptions) (client.NetworkRemoveResult, error) {
	return client.NetworkRemoveResult{}, errors.New("unexpected NetworkRemove")
}

func TestPermanentlyHungDockerTransportIsBounded(t *testing.T) {
	transport := newHungDockerTransport()
	executor := &Executor{client: transport, nodeID: "node", controlPlaneTimeout: 25 * time.Millisecond}
	result := make(chan error, 1)
	go func() {
		_, err := executor.Managed(context.Background())
		result <- err
	}()
	<-transport.entered

	deadline, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case err := <-result:
		require.ErrorContains(t, err, "Docker deadline exceeded")
	case <-deadline.Done():
		t.Fatal("hung Docker transport exceeded its control-plane deadline")
	}
	select {
	case <-transport.cancelled:
	case <-deadline.Done():
		t.Fatal("Docker transport did not observe deadline cancellation")
	}
}

func TestDockerTransportPropagatesCallerCancellation(t *testing.T) {
	transport := newHungDockerTransport()
	executor := &Executor{client: transport, nodeID: "node", controlPlaneTimeout: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := executor.Managed(ctx)
		result <- err
	}()
	<-transport.entered
	cancel()

	deadline, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	select {
	case err := <-result:
		require.ErrorContains(t, err, "Docker cancelled")
	case <-deadline.Done():
		t.Fatal("Docker transport ignored caller cancellation")
	}
}

func TestFailedStartCleanupPropagatesCallerCancellation(t *testing.T) {
	transport := newHungDockerTransport()
	transport.hangStop = true
	executor := &Executor{client: transport, nodeID: "node", stopTimeout: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := executor.failCreatedContainer(ctx, "created", errors.New("start failed"))
		result <- err
	}()
	<-transport.entered
	cancel()

	select {
	case err := <-result:
		require.ErrorContains(t, err, "start failed")
	case <-time.After(time.Second):
		t.Fatal("failed-start cleanup ignored caller cancellation")
	}
	select {
	case <-transport.cancelled:
	case <-time.After(time.Second):
		t.Fatal("failed-start Docker cleanup did not observe cancellation")
	}
}
