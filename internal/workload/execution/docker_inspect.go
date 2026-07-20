package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/client"
)

func (e *DockerExecutor) list(ctx context.Context, workloadID string) ([]Instance, error) {
	filters := client.Filters{}
	filters = filters.Add("label", labelManaged+"=true")
	filters = filters.Add("label", labelNode+"="+e.nodeID)
	if workloadID != "" {
		filters = filters.Add("label", labelWorkload+"="+workloadID)
	}
	result, err := e.client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: filters})
	if err != nil {
		return nil, dockerSafeError("list workload containers", err)
	}
	instances := make([]Instance, 0, len(result.Items))
	for _, item := range result.Items {
		instance, err := e.inspectID(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

func (e *DockerExecutor) Managed(ctx context.Context) ([]Instance, error) {
	return e.list(ctx, "")
}

func (e *DockerExecutor) inspectID(ctx context.Context, id string) (Instance, error) {
	result, err := e.client.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return Instance{}, dockerSafeError("inspect workload container", err)
	}
	container := result.Container
	if container.Config == nil || container.State == nil {
		return Instance{}, fmt.Errorf("container %s returned incomplete inspection state", id)
	}
	labels := container.Config.Labels
	if labels[labelManaged] != "true" || labels[labelNode] != e.nodeID || labels[labelWorkload] == "" {
		return Instance{}, fmt.Errorf("container %s does not have a valid Ardents ownership identity", id)
	}
	if labels[labelRuntime] == "" || (labels[labelTrust] != "trusted" && labels[labelTrust] != "untrusted") {
		return Instance{}, fmt.Errorf("container %s has incomplete Ardents runtime identity", id)
	}
	generation, err := generationFromLabels(labels)
	if err != nil {
		return Instance{}, err
	}
	state := container.State
	instance := Instance{WorkloadID: labels[labelWorkload], Generation: generation, RuntimeID: container.ID,
		Running: state.Running, PID: state.Pid, OOMKilled: state.OOMKilled, Restarts: container.RestartCount,
		Runtime: labels[labelRuntime], TrustClass: labels[labelTrust]}
	if container.HostConfig != nil {
		instance.MemoryLimitBytes = container.HostConfig.Memory
		instance.NanoCPUs = container.HostConfig.NanoCPUs
		if container.HostConfig.PidsLimit != nil {
			instance.PIDsLimit = *container.HostConfig.PidsLimit
		}
	}
	instance.StartedAt = parseDockerTime(state.StartedAt)
	instance.FinishedAt = parseDockerTime(state.FinishedAt)
	if !state.Running {
		code := state.ExitCode
		instance.ExitCode = &code
		instance.Reason = containerExitReason(state.OOMKilled, state.Error, string(state.Status))
	}
	return instance, nil
}

func parseDockerTime(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || parsed.Year() <= 1 {
		return time.Time{}
	}
	return parsed.UTC()
}

func containerExitReason(oom bool, stateError, status string) string {
	if oom {
		return "oom_killed"
	}
	if stateError != "" {
		return stateError
	}
	if status == "dead" {
		return "container_dead"
	}
	return "exited"
}
