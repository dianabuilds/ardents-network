package docker

import (
	"ardents/internal/ingressproxy"
	"ardents/internal/workload/execution"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	containerapi "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func (e *Executor) ensureIngressProxy(ctx context.Context, prepared execution.PreparedWorkload, workloadID string) error {
	items, err := e.ingressProxyContainers(ctx, prepared)
	if err != nil {
		return err
	}
	expectedImageID, validationErr := e.validateIngressProxyImage(ctx)
	if validationErr != nil {
		for _, item := range items {
			validationErr = errors.Join(validationErr, e.stopAndRemoveContainer(ctx, item.ID))
		}
		return validationErr
	}
	if len(items) > 1 {
		duplicateErr := error(fmt.Errorf("workload ingress proxy identity is duplicated"))
		for _, item := range items {
			duplicateErr = errors.Join(duplicateErr, e.stopAndRemoveContainer(ctx, item.ID))
		}
		return duplicateErr
	}
	if len(items) == 1 {
		if items[0].State == "running" && items[0].ImageID == expectedImageID {
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
		_, cleanupErr := e.client.ContainerRemove(context.Background(), created.ID, client.ContainerRemoveOptions{Force: true})
		startErr := dockerSafeError("start workload ingress proxy", err)
		if cleanupErr == nil {
			return startErr
		}
		return errors.Join(startErr, dockerSafeError("remove failed workload ingress proxy", cleanupErr))
	}
	return nil
}

func (e *Executor) validateIngressProxyImage(ctx context.Context) (string, error) {
	inspected, err := e.client.ImageInspect(ctx, e.ingressProxyImage)
	if err != nil {
		return "", dockerSafeError("inspect workload ingress proxy image", err)
	}
	if inspected.Config == nil {
		return "", fmt.Errorf("workload ingress proxy image has no configuration")
	}
	if err := validateIngressProxyLabels(inspected.Config.Labels); err != nil {
		return "", err
	}
	if inspected.ID == "" {
		return "", fmt.Errorf("workload ingress proxy image has no immutable identity")
	}
	return inspected.ID, nil
}

func validateIngressProxyLabels(labels map[string]string) error {
	if labels[ingressproxy.ProtocolLabel] != ingressproxy.ProtocolVersion() {
		return fmt.Errorf("workload ingress proxy protocol is incompatible")
	}
	return nil
}

func (e *Executor) ingressProxyCreateOptions(prepared execution.PreparedWorkload, workloadID string) client.ContainerCreateOptions {
	ingress := e.proxyIngressOptions(prepared)
	return client.ContainerCreateOptions{
		Name: proxyContainerName(prepared),
		Config: &containerapi.Config{Image: e.ingressProxyImage, User: "65534:65534",
			Entrypoint: []string{"/ardents-ingress-proxy"}, Cmd: proxyCommand(prepared),
			Labels: proxyLabels(e.nodeID, prepared, workloadID), ExposedPorts: ingress.exposedPorts},
		HostConfig: &containerapi.HostConfig{NetworkMode: ingress.networkMode, PortBindings: ingress.portBindings,
			Runtime: e.trustedRuntime, ReadonlyRootfs: true, CapDrop: []string{"ALL"}, Init: new(true),
			SecurityOpt: []string{"no-new-privileges:true"}, AutoRemove: false,
			LogConfig: containerapi.LogConfig{Type: "local", Config: map[string]string{"max-size": "2m", "max-file": "2"}},
			Resources: containerapi.Resources{Memory: 64 * 1024 * 1024, MemorySwap: 64 * 1024 * 1024,
				NanoCPUs: 250_000_000, PidsLimit: new(int64(32))}},
		NetworkingConfig: ingress.networking,
	}
}

func proxyCommand(prepared execution.PreparedWorkload) []string {
	ports := make([]string, 0, len(prepared.Ingress))
	for _, binding := range prepared.Ingress {
		ports = append(ports, strconv.Itoa(int(binding.Port)))
	}
	return []string{"--target", containerName(prepared.WorkloadID, prepared.Generation), "--ports", strings.Join(ports, ",")}
}

func proxyLabels(nodeID string, prepared execution.PreparedWorkload, workloadID string) map[string]string {
	return map[string]string{labelProxy: "true", labelNode: nodeID, labelWorkload: prepared.WorkloadID,
		labelGeneration: strconv.FormatInt(prepared.Generation, 10), "io.ardents.backing-container": workloadID}
}

func proxyContainerName(prepared execution.PreparedWorkload) string {
	return containerName(prepared.WorkloadID+"-ingress", prepared.Generation)
}

type proxyContainer struct {
	ID         string
	ImageID    string
	State      string
	WorkloadID string
	Generation int64
}

func (e *Executor) ingressProxyContainers(ctx context.Context, prepared execution.PreparedWorkload) ([]proxyContainer, error) {
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
		items = append(items, proxyContainer{ID: item.ID, ImageID: item.ImageID, State: string(item.State),
			WorkloadID: item.Labels[labelWorkload], Generation: generation})
	}
	return items, nil
}

func (e *Executor) allIngressProxyContainers(ctx context.Context) ([]proxyContainer, error) {
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
		items = append(items, proxyContainer{ID: item.ID, ImageID: item.ImageID, State: string(item.State),
			WorkloadID: item.Labels[labelWorkload], Generation: generation})
	}
	return items, nil
}

func (e *Executor) stopAndRemoveIngressProxy(ctx context.Context, instance execution.Instance) error {
	prepared := execution.PreparedWorkload{WorkloadID: instance.WorkloadID, Generation: instance.Generation}
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

func (e *Executor) stopAndRemoveContainer(ctx context.Context, id string) error {
	if _, err := e.client.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: new(int(e.stopTimeout.Seconds()))}); err != nil && !cerrdefs.IsNotFound(err) && !cerrdefs.IsNotModified(err) {
		return dockerSafeError("stop managed container", err)
	}
	if _, err := e.client.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !cerrdefs.IsNotFound(err) {
		return dockerSafeError("remove managed container", err)
	}
	return nil
}
