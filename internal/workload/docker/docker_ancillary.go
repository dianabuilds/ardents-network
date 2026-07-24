// Package docker owns the Docker Engine execution adapter.
// It does not own workload policy or desired-state ownership.
package docker

import (
	"ardents/internal/workload/execution"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/moby/moby/client"
)

func encodeIngressLabel(bindings []execution.IngressBinding) string {
	if len(bindings) == 0 {
		return ""
	}
	raw, err := json.Marshal(bindings)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeIngressLabel(encoded string) ([]execution.IngressBinding, error) {
	if encoded == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid workload ingress identity")
	}
	var bindings []execution.IngressBinding
	if err := json.Unmarshal(raw, &bindings); err != nil || len(bindings) == 0 {
		return nil, fmt.Errorf("invalid workload ingress identity")
	}
	return bindings, nil
}

func (e *Executor) ReconcileAncillary(ctx context.Context, current []execution.Instance) error {
	ctx, cancel := e.controlContext(ctx)
	defer cancel()
	active := make(map[string]execution.Instance, len(current))
	for _, instance := range current {
		if !instance.Running {
			continue
		}
		active[ancillaryIdentity(instance.WorkloadID, instance.Generation)] = instance
	}
	proxies, err := e.allIngressProxyContainers(ctx)
	if err != nil {
		return err
	}
	for _, proxy := range proxies {
		if _, ok := active[ancillaryIdentity(proxy.WorkloadID, proxy.Generation)]; ok {
			continue
		}
		if err := e.stopAndRemoveContainer(ctx, proxy.ID); err != nil {
			return err
		}
	}
	for _, instance := range current {
		if !instance.Running {
			continue
		}
		if err := e.recoverInstanceIngress(ctx, instance); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) recoverInstanceIngress(ctx context.Context, instance execution.Instance) error {
	result, err := e.client.ContainerInspect(ctx, instance.RuntimeID, client.ContainerInspectOptions{})
	if err != nil {
		return dockerSafeError("inspect workload ingress identity", err)
	}
	bindings, err := decodeIngressLabel(result.Container.Config.Labels[labelIngress])
	if err != nil || len(bindings) == 0 {
		return err
	}
	prepared := execution.PreparedWorkload{WorkloadID: instance.WorkloadID, Generation: instance.Generation, Ingress: bindings}
	if err := e.ensureIngressNetworks(ctx, prepared); err != nil {
		return err
	}
	return e.ensureIngressProxy(ctx, prepared, instance.RuntimeID)
}

func ancillaryIdentity(workloadID string, generation int64) string {
	return fmt.Sprintf("%s\x00%d", workloadID, generation)
}
