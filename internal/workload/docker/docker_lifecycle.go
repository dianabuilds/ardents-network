package docker

import (
	"ardents/internal/workload/execution"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

func (e *Executor) failCreatedContainer(id string, cause error) (execution.Instance, error) {
	stopTimeout := e.stopTimeout
	if stopTimeout <= 0 {
		stopTimeout = 10 * time.Second
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), stopTimeout+5*time.Second)
	defer cancel()
	cleanupErr := e.stopAndRemoveContainer(cleanupCtx, id)
	return execution.Instance{}, errors.Join(cause, cleanupErr)
}

func (e *Executor) Inspect(ctx context.Context, workloadID string) (execution.Instance, error) {
	instances, err := e.list(ctx, workloadID)
	if err != nil {
		return execution.Instance{}, err
	}
	if len(instances) == 0 {
		return execution.Instance{}, fmt.Errorf("workload %s not found", workloadID)
	}
	sort.Slice(instances, func(i, j int) bool { return instances[i].Generation > instances[j].Generation })
	if len(instances) > 1 && instances[0].Generation == instances[1].Generation {
		return execution.Instance{}, fmt.Errorf("workload %s has duplicate current generation", workloadID)
	}
	return instances[0], nil
}

func (e *Executor) Stop(ctx context.Context, instance execution.Instance) error {
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
	if _, err := e.client.ContainerStop(ctx, target, client.ContainerStopOptions{Timeout: new(int(e.stopTimeout / time.Second))}); err != nil && !errdefs.IsNotFound(err) {
		return dockerSafeError("stop workload container", err)
	}
	return nil
}

func (e *Executor) Remove(ctx context.Context, instance execution.Instance) error {
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
	prepared := execution.PreparedWorkload{WorkloadID: instance.WorkloadID, Generation: instance.Generation}
	_, networkErr := e.client.NetworkRemove(ctx, e.workloadNetworkName(prepared), client.NetworkRemoveOptions{})
	if networkErr != nil && !errdefs.IsNotFound(networkErr) {
		return dockerSafeError("remove workload internal network", networkErr)
	}
	return nil
}

func (e *Executor) runtimeID(ctx context.Context, instance execution.Instance) (string, bool, error) {
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

func (e *Executor) findGeneration(ctx context.Context, workloadID string, generation int64) (execution.Instance, bool, error) {
	instances, err := e.list(ctx, workloadID)
	if err != nil {
		return execution.Instance{}, false, err
	}
	var found []execution.Instance
	for _, instance := range instances {
		if instance.Generation == generation {
			found = append(found, instance)
		}
	}
	if len(found) > 1 {
		return execution.Instance{}, false, fmt.Errorf("workload %s generation %d has duplicate containers", workloadID, generation)
	}
	if len(found) == 0 {
		return execution.Instance{}, false, nil
	}
	return found[0], true, nil
}
