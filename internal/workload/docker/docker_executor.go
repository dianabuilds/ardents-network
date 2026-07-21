package docker

import (
	"ardents/internal/workload/execution"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	containerapi "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type ExecutorConfig struct {
	NodeID              string
	Runtime             string
	TrustedRuntime      string
	UntrustedRuntime    string
	AllowedRegistries   []string
	AllowedPolicyRefs   []string
	AllowedIngressHosts []string
	IngressBindAddress  string
	IngressProxyImage   string
	AllowInsecureRemote bool
	StopTimeout         time.Duration
}

type Executor struct {
	client              *client.Client
	nodeID              string
	trustedRuntime      string
	untrustedRuntime    string
	allowedRegistries   map[string]struct{}
	allowedPolicyRefs   map[string]struct{}
	allowedIngressHosts map[string]struct{}
	ingressBindAddress  string
	ingressNetworkName  string
	ingressProxyImage   string
	stopTimeout         time.Duration
}

func NewExecutor(cfg ExecutorConfig) (*Executor, error) {
	if cfg.NodeID == "" {
		return nil, fmt.Errorf("docker executor requires node id")
	}
	engine, err := client.New(client.FromEnv)
	if err != nil {
		return nil, dockerSafeError("initialize Docker Engine client", err)
	}
	if err := validateDockerEndpoint(engine.DaemonHost(), cfg.AllowInsecureRemote); err != nil {
		return nil, errors.Join(err, engine.Close())
	}
	if cfg.Runtime != "" {
		cfg.TrustedRuntime = cfg.Runtime
		cfg.UntrustedRuntime = cfg.Runtime
	}
	if cfg.TrustedRuntime == "" {
		cfg.TrustedRuntime = "runc"
	}
	if cfg.UntrustedRuntime == "" {
		cfg.UntrustedRuntime = "runsc"
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 10 * time.Second
	}
	return &Executor{
		client: engine, nodeID: cfg.NodeID, trustedRuntime: cfg.TrustedRuntime,
		untrustedRuntime: cfg.UntrustedRuntime, allowedRegistries: normalizedSet(cfg.AllowedRegistries),
		allowedPolicyRefs: normalizedSet(cfg.AllowedPolicyRefs), allowedIngressHosts: normalizedSet(cfg.AllowedIngressHosts),
		ingressBindAddress: strings.TrimSpace(cfg.IngressBindAddress), ingressNetworkName: dockerIngressNetworkName(cfg.NodeID),
		ingressProxyImage: strings.TrimSpace(cfg.IngressProxyImage),
		stopTimeout:       cfg.StopTimeout,
	}, nil
}

func (e *Executor) Prepare(_ context.Context, req execution.Request) (execution.PreparedWorkload, error) {
	spec, err := parseContainerSpec(req.Config)
	if err != nil {
		return execution.PreparedWorkload{}, err
	}
	if err := e.admitImage(spec.Image); err != nil {
		return execution.PreparedWorkload{}, err
	}
	if err := e.admitPolicyRef(req.PolicyRef); err != nil {
		return execution.PreparedWorkload{}, err
	}
	handle, err := encodeContainerSpec(spec)
	if err != nil {
		return execution.PreparedWorkload{}, err
	}
	now := time.Now().UTC()
	ingress, err := e.admitIngress(req.Ingress)
	if err != nil {
		return execution.PreparedWorkload{}, err
	}
	return execution.PreparedWorkload{WorkloadID: req.WorkloadID, Generation: now.UnixNano(), PreparedAt: now,
		Handle: handle, PolicyRef: strings.ToLower(strings.TrimSpace(req.PolicyRef)), Ingress: ingress}, nil
}

func (e *Executor) Start(ctx context.Context, prepared execution.PreparedWorkload) (execution.Instance, error) {
	spec, err := parseContainerSpec(prepared.Handle)
	if err != nil {
		return execution.Instance{}, err
	}
	if err := e.admitImage(spec.Image); err != nil {
		return execution.Instance{}, err
	}
	if err := e.admitPolicyRef(prepared.PolicyRef); err != nil {
		return execution.Instance{}, err
	}
	if len(prepared.Ingress) > 0 {
		if err := e.ensureIngressNetworks(ctx, prepared); err != nil {
			return execution.Instance{}, err
		}
	}
	existing, found, err := e.findGeneration(ctx, prepared.WorkloadID, prepared.Generation)
	if err != nil {
		return execution.Instance{}, err
	}
	if found {
		return e.startExisting(ctx, prepared, existing)
	}
	return e.startNew(ctx, spec, prepared)
}

func (e *Executor) startNew(ctx context.Context, spec containerSpec, prepared execution.PreparedWorkload) (execution.Instance, error) {
	created, err := e.client.ContainerCreate(ctx, e.createOptions(spec, prepared))
	if err != nil {
		return execution.Instance{}, dockerSafeError("create workload container", err)
	}
	if _, err := e.client.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		return e.failCreatedContainer(created.ID, dockerSafeError("start workload container", err))
	}
	instance, err := e.inspectID(ctx, created.ID)
	if err != nil {
		return e.failCreatedContainer(created.ID, err)
	}
	if len(prepared.Ingress) > 0 {
		if err := e.ensureIngressProxy(ctx, prepared, created.ID); err != nil {
			return e.failCreatedContainer(created.ID, err)
		}
	}
	return instance, nil
}

func (e *Executor) startExisting(ctx context.Context, prepared execution.PreparedWorkload, existing execution.Instance) (execution.Instance, error) {
	runtime, trustClass := e.executionClass(prepared.PolicyRef)
	if existing.Runtime != runtime || existing.TrustClass != trustClass {
		return execution.Instance{}, fmt.Errorf("workload runtime identity conflicts with admitted policy")
	}
	if !existing.Running {
		if _, err := e.client.ContainerStart(ctx, existing.RuntimeID, client.ContainerStartOptions{}); err != nil {
			return execution.Instance{}, dockerSafeError("start existing workload container", err)
		}
	}
	if len(prepared.Ingress) > 0 {
		if err := e.ensureIngressProxy(ctx, prepared, existing.RuntimeID); err != nil {
			return execution.Instance{}, err
		}
	}
	return e.inspectID(ctx, existing.RuntimeID)
}

func (e *Executor) createOptions(spec containerSpec, prepared execution.PreparedWorkload) client.ContainerCreateOptions {
	runtime, trustClass := e.executionClass(prepared.PolicyRef)
	ingress := e.workloadIngressOptions(prepared)
	labels := workloadLabels(e.nodeID, prepared.WorkloadID, runtime, trustClass, prepared.Generation)
	if encoded := encodeIngressLabel(prepared.Ingress); encoded != "" {
		labels[labelIngress] = encoded
	}
	return client.ContainerCreateOptions{
		Name: containerName(prepared.WorkloadID, prepared.Generation),
		Config: &containerapi.Config{
			Image: spec.Image, Cmd: spec.Command, Entrypoint: spec.Entrypoint,
			Env: runtimeEnvironment(spec.Env, prepared.Generation), WorkingDir: spec.WorkingDir, User: spec.User,
			Labels:          labels,
			NetworkDisabled: len(prepared.Ingress) == 0,
		},
		HostConfig: &containerapi.HostConfig{
			NetworkMode: ingress.networkMode,
			Runtime:     runtime, ReadonlyRootfs: true, CapDrop: []string{"ALL"},
			SecurityOpt: []string{"no-new-privileges:true"}, AutoRemove: false, Init: new(true),
			LogConfig: containerapi.LogConfig{Type: "local", Config: map[string]string{"max-size": "10m", "max-file": "2"}},
			Tmpfs:     map[string]string{"/tmp": tmpfsOptions(spec.Resources.TmpfsBytes)},
			Resources: containerapi.Resources{Memory: spec.Resources.MemoryBytes, MemorySwap: spec.Resources.MemoryBytes,
				NanoCPUs: spec.Resources.NanoCPUs, PidsLimit: new(spec.Resources.PIDs)},
		},
		NetworkingConfig: ingress.networking,
	}
}

func (e *Executor) executionClass(policyRef string) (string, string) {
	if strings.EqualFold(strings.TrimSpace(policyRef), "trusted") {
		return e.trustedRuntime, "trusted"
	}
	return e.untrustedRuntime, "untrusted"
}

func (e *Executor) admitPolicyRef(policyRef string) error {
	normalized := strings.ToLower(strings.TrimSpace(policyRef))
	if normalized == "" {
		return nil
	}
	if _, allowed := e.allowedPolicyRefs[normalized]; !allowed {
		return fmt.Errorf("workload policy reference is not allowed")
	}
	return nil
}

func normalizedSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}
