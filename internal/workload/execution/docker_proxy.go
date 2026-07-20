package execution

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	containerapi "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func (e *DockerExecutor) ensureIngressProxy(ctx context.Context, prepared PreparedWorkload, workloadID string) error {
	items, err := e.ingressProxyContainers(ctx, prepared)
	if err != nil {
		return err
	}
	if len(items) > 1 {
		return fmt.Errorf("workload ingress proxy identity is duplicated")
	}
	if len(items) == 1 {
		if items[0].State == "running" {
			return nil
		}
		if _, err := e.client.ContainerRemove(ctx, items[0].ID, client.ContainerRemoveOptions{Force: true}); err != nil {
			return dockerSafeError("replace stopped workload ingress proxy", err)
		}
	}
	options := e.ingressProxyCreateOptions(prepared, workloadID)
	created, err := e.client.ContainerCreate(ctx, options)
	if err != nil {
		return dockerSafeError("create workload ingress proxy", err)
	}
	if _, err := e.client.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		_, _ = e.client.ContainerRemove(context.Background(), created.ID, client.ContainerRemoveOptions{Force: true})
		return dockerSafeError("start workload ingress proxy", err)
	}
	return nil
}

func (e *DockerExecutor) ingressProxyCreateOptions(prepared PreparedWorkload, workloadID string) client.ContainerCreateOptions {
	init := true
	pids := int64(32)
	ingress := e.proxyIngressOptions(prepared)
	return client.ContainerCreateOptions{
		Name: proxyContainerName(prepared),
		Config: &containerapi.Config{Image: e.ingressProxyImage, User: "65534:65534",
			Entrypoint: []string{"/ardents-ingress-proxy"}, Cmd: proxyCommand(prepared),
			Labels: proxyLabels(e.nodeID, prepared, workloadID), ExposedPorts: ingress.exposedPorts},
		HostConfig: &containerapi.HostConfig{NetworkMode: ingress.networkMode, PortBindings: ingress.portBindings,
			Runtime: e.trustedRuntime, ReadonlyRootfs: true, CapDrop: []string{"ALL"}, Init: &init,
			SecurityOpt: []string{"no-new-privileges:true"}, AutoRemove: false,
			LogConfig: containerapi.LogConfig{Type: "local", Config: map[string]string{"max-size": "2m", "max-file": "2"}},
			Resources: containerapi.Resources{Memory: 64 * 1024 * 1024, MemorySwap: 64 * 1024 * 1024,
				NanoCPUs: 250_000_000, PidsLimit: &pids}},
		NetworkingConfig: ingress.networking,
	}
}

func proxyCommand(prepared PreparedWorkload) []string {
	ports := make([]string, 0, len(prepared.Ingress))
	for _, binding := range prepared.Ingress {
		ports = append(ports, strconv.Itoa(int(binding.Port)))
	}
	return []string{"--target", containerName(prepared.WorkloadID, prepared.Generation), "--ports", strings.Join(ports, ",")}
}

func proxyLabels(nodeID string, prepared PreparedWorkload, workloadID string) map[string]string {
	return map[string]string{labelProxy: "true", labelNode: nodeID, labelWorkload: prepared.WorkloadID,
		labelGeneration: strconv.FormatInt(prepared.Generation, 10), "io.ardents.backing-container": workloadID}
}

func proxyContainerName(prepared PreparedWorkload) string {
	return containerName(prepared.WorkloadID+"-ingress", prepared.Generation)
}

type proxyContainer struct {
	ID         string
	State      string
	WorkloadID string
	Generation int64
}

func (e *DockerExecutor) ingressProxyContainers(ctx context.Context, prepared PreparedWorkload) ([]proxyContainer, error) {
	filters := client.Filters{}
	filters = filters.Add("label", labelProxy+"=true")
	filters = filters.Add("label", labelNode+"="+e.nodeID)
	filters = filters.Add("label", labelWorkload+"="+prepared.WorkloadID)
	filters = filters.Add("label", labelGeneration+"="+strconv.FormatInt(prepared.Generation, 10))
	result, err := e.client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: filters})
	if err != nil {
		return nil, dockerSafeError("list workload ingress proxies", err)
	}
	items := make([]proxyContainer, 0, len(result.Items))
	for _, item := range result.Items {
		generation, parseErr := generationFromLabels(item.Labels)
		if parseErr != nil || item.Labels[labelWorkload] == "" {
			return nil, fmt.Errorf("workload ingress proxy has invalid ownership identity")
		}
		items = append(items, proxyContainer{ID: item.ID, State: string(item.State),
			WorkloadID: item.Labels[labelWorkload], Generation: generation})
	}
	return items, nil
}

func (e *DockerExecutor) allIngressProxyContainers(ctx context.Context) ([]proxyContainer, error) {
	filters := client.Filters{}
	filters = filters.Add("label", labelProxy+"=true")
	filters = filters.Add("label", labelNode+"="+e.nodeID)
	result, err := e.client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: filters})
	if err != nil {
		return nil, dockerSafeError("list workload ingress proxies", err)
	}
	items := make([]proxyContainer, 0, len(result.Items))
	for _, item := range result.Items {
		generation, parseErr := generationFromLabels(item.Labels)
		if parseErr != nil || item.Labels[labelWorkload] == "" {
			return nil, fmt.Errorf("workload ingress proxy has invalid ownership identity")
		}
		items = append(items, proxyContainer{ID: item.ID, State: string(item.State),
			WorkloadID: item.Labels[labelWorkload], Generation: generation})
	}
	return items, nil
}

func (e *DockerExecutor) stopAndRemoveIngressProxy(ctx context.Context, instance Instance) error {
	prepared := PreparedWorkload{WorkloadID: instance.WorkloadID, Generation: instance.Generation}
	items, err := e.ingressProxyContainers(ctx, prepared)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := e.stopAndRemoveContainer(ctx, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (e *DockerExecutor) stopAndRemoveContainer(ctx context.Context, id string) error {
	timeout := int(e.stopTimeout.Seconds())
	if _, err := e.client.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &timeout}); err != nil && !cerrdefs.IsNotFound(err) && !cerrdefs.IsNotModified(err) {
		return dockerSafeError("stop managed container", err)
	}
	if _, err := e.client.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !cerrdefs.IsNotFound(err) {
		return dockerSafeError("remove managed container", err)
	}
	return nil
}
