package execution

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

func (e *DockerExecutor) Inspect(ctx context.Context, workloadID string) (Instance, error) {
	instances, err := e.list(ctx, workloadID)
	if err != nil {
		return Instance{}, err
	}
	if len(instances) == 0 {
		return Instance{}, fmt.Errorf("workload %s not found", workloadID)
	}
	sort.Slice(instances, func(i, j int) bool { return instances[i].Generation > instances[j].Generation })
	if len(instances) > 1 && instances[0].Generation == instances[1].Generation {
		return Instance{}, fmt.Errorf("workload %s has duplicate current generation", workloadID)
	}
	return instances[0], nil
}

func (e *DockerExecutor) Stop(ctx context.Context, instance Instance) error {
	if err := e.stopAndRemoveIngressProxy(ctx, instance); err != nil {
		return err
	}
	target, found, err := e.runtimeID(ctx, instance)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	timeout := int(e.stopTimeout / time.Second)
	if _, err := e.client.ContainerStop(ctx, target, client.ContainerStopOptions{Timeout: &timeout}); err != nil && !errdefs.IsNotFound(err) {
		return dockerSafeError("stop workload container", err)
	}
	return nil
}

func (e *DockerExecutor) Remove(ctx context.Context, instance Instance) error {
	if err := e.stopAndRemoveIngressProxy(ctx, instance); err != nil {
		return err
	}
	target, found, err := e.runtimeID(ctx, instance)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	_, err = e.client.ContainerRemove(ctx, target, client.ContainerRemoveOptions{Force: false, RemoveVolumes: true})
	if err != nil && !errdefs.IsNotFound(err) {
		return dockerSafeError("remove workload container", err)
	}
	prepared := PreparedWorkload{WorkloadID: instance.WorkloadID, Generation: instance.Generation}
	_, networkErr := e.client.NetworkRemove(ctx, e.workloadNetworkName(prepared), client.NetworkRemoveOptions{})
	if networkErr != nil && !errdefs.IsNotFound(networkErr) {
		return dockerSafeError("remove workload internal network", networkErr)
	}
	return nil
}

func (e *DockerExecutor) runtimeID(ctx context.Context, instance Instance) (string, bool, error) {
	if instance.RuntimeID != "" {
		return instance.RuntimeID, true, nil
	}
	found, ok, err := e.findGeneration(ctx, instance.WorkloadID, instance.Generation)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return found.RuntimeID, true, nil
}

func (e *DockerExecutor) findGeneration(ctx context.Context, workloadID string, generation int64) (Instance, bool, error) {
	instances, err := e.list(ctx, workloadID)
	if err != nil {
		return Instance{}, false, err
	}
	var found []Instance
	for _, instance := range instances {
		if instance.Generation == generation {
			found = append(found, instance)
		}
	}
	if len(found) > 1 {
		return Instance{}, false, fmt.Errorf("workload %s generation %d has duplicate containers", workloadID, generation)
	}
	if len(found) == 0 {
		return Instance{}, false, nil
	}
	return found[0], true, nil
}
