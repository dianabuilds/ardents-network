//go:build integration

package docker_test

import (
	workloadregistry "ardents/internal/workload/registry"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	workloaddocker "ardents/internal/workload/docker"
	"ardents/internal/workload/execution"
	workloadcontroller "ardents/internal/workload/execution"
	"ardents/tests/testkit"

	containerapi "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

func TestDockerExecutorLifecycleIsIdempotentAndRecoverable(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-002", Suite: "integration", Tags: []string{"integration", "workload", "docker", "recovery"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	nodeID := fmt.Sprintf("stb402-%d", time.Now().UnixNano())
	executor := newDockerExecutor(t, nodeID)
	prepared, err := executor.Prepare(ctx, execution.Request{
		WorkloadID: "work.docker.lifecycle",
		Config:     dockerConfig(t, []string{"sh", "-c", "while true; do sleep 1; done"}),
	})
	require.NoError(t, err)
	instance, err := executor.Start(ctx, prepared)
	require.NoError(t, err)
	cleanupDockerInstance(t, executor, instance)
	require.True(t, instance.Running)
	require.NotEmpty(t, instance.RuntimeID)

	duplicate, err := executor.Start(ctx, prepared)
	require.NoError(t, err)
	require.Equal(t, instance.RuntimeID, duplicate.RuntimeID)

	recoveredExecutor := newDockerExecutor(t, nodeID)
	recovered, err := recoveredExecutor.Inspect(ctx, prepared.WorkloadID)
	require.NoError(t, err)
	require.Equal(t, prepared.Generation, recovered.Generation)
	require.Equal(t, instance.RuntimeID, recovered.RuntimeID)
	require.True(t, recovered.Running)

	require.NoError(t, recoveredExecutor.Stop(ctx, recovered))
	stopped := waitForDockerStopped(t, recoveredExecutor, prepared.WorkloadID)
	require.False(t, stopped.Running)
	require.NotNil(t, stopped.ExitCode)
	require.NoError(t, recoveredExecutor.Remove(ctx, stopped))
	_, err = recoveredExecutor.Inspect(ctx, prepared.WorkloadID)
	require.ErrorContains(t, err, "not found")
}

func TestDockerExecutorRetainsCrashOutcomeAndRejectsMutableImage(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-002", Suite: "integration", Tags: []string{"integration", "workload", "docker", "recovery"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	executor := newDockerExecutor(t, fmt.Sprintf("stb402-%d", time.Now().UnixNano()))
	_, err := executor.Prepare(ctx, execution.Request{WorkloadID: "work.mutable", Config: `{"image":"busybox:latest","user":"65534"}`})
	require.ErrorContains(t, err, "immutable sha256 digest")
	missingImage := fmt.Sprintf(`{"image":"docker.io/library/ardents-missing@sha256:%s","user":"65534:65534"}`, strings.Repeat("a", 64))
	missing, err := executor.Prepare(ctx, execution.Request{WorkloadID: "work.missing", Config: missingImage})
	require.NoError(t, err)
	_, err = executor.Start(ctx, missing)
	require.ErrorContains(t, err, "create workload container")

	prepared, err := executor.Prepare(ctx, execution.Request{
		WorkloadID: "work.docker.crash",
		Config:     dockerConfig(t, []string{"sh", "-c", "exit 7"}),
	})
	require.NoError(t, err)
	instance, err := executor.Start(ctx, prepared)
	require.NoError(t, err)
	cleanupDockerInstance(t, executor, instance)
	stopped := waitForDockerStopped(t, executor, prepared.WorkloadID)
	require.NotNil(t, stopped.ExitCode)
	require.Equal(t, 7, *stopped.ExitCode)
	require.Equal(t, "exited", stopped.Reason)
	assertDockerStartFailureExhaustsRestartBudget(t, ctx, missingImage)
}

func assertDockerStartFailureExhaustsRestartBudget(t *testing.T, ctx context.Context, config string) {
	t.Helper()
	adapter := newControllerDockerExecutor(t, fmt.Sprintf("stb402-budget-%d", time.Now().UnixNano()))
	service := workloadcontroller.New(t.TempDir()+"/ardents.db", adapter)
	require.NoError(t, service.Load())
	require.NoError(t, service.Register(workloadregistry.Spec{
		ID: "work.docker.budget", Kind: "service", Owner: "node", Config: config,
		Desired: workloadregistry.DesiredRunning, PolicyRef: "trusted",
	}))
	for range workloadcontroller.DefaultRestartBudget + 1 {
		require.NoError(t, service.Reconcile(ctx))
	}
	status, ok := service.Get("work.docker.budget")
	require.True(t, ok)
	require.Equal(t, workloadcontroller.ObservedFailed, status.Observed)
	require.True(t, status.NeedsOperatorAction)
	require.Equal(t, workloadcontroller.DefaultRestartBudget+1, status.RestartCount)
}

func TestDockerExecutorControllerRecoversAndRemovesObservedInstance(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-002", Suite: "integration", Tags: []string{"integration", "workload", "docker", "recovery"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	nodeID := fmt.Sprintf("stb402-controller-%d", time.Now().UnixNano())
	adapter := newControllerDockerExecutor(t, nodeID)
	path := t.TempDir() + "/ardents.db"
	service := workloadcontroller.New(path, adapter)
	require.NoError(t, service.Load())
	require.NoError(t, service.Register(workloadregistry.Spec{
		ID: "work.controller.recovery", Kind: "service", Owner: "node",
		Config:  dockerConfig(t, []string{"sh", "-c", "while true; do sleep 1; done"}),
		Desired: workloadregistry.DesiredRunning, PolicyRef: "trusted",
	}))
	require.NoError(t, service.Reconcile(ctx))
	started, ok := service.Get("work.controller.recovery")
	require.True(t, ok)
	require.True(t, started.Instance.Running)

	recoveredAdapter := newControllerDockerExecutor(t, nodeID)
	recovered := workloadcontroller.New(path, recoveredAdapter)
	require.NoError(t, recovered.Load())
	current, ok := recovered.Get("work.controller.recovery")
	require.True(t, ok)
	require.True(t, current.Instance.Running)
	require.Equal(t, started.Instance.RuntimeID, current.Instance.RuntimeID)

	require.NoError(t, recovered.SetDesired("work.controller.recovery", workloadregistry.DesiredRemoved))
	require.NoError(t, recovered.Reconcile(ctx))
	_, ok = recovered.Get("work.controller.recovery")
	require.False(t, ok)
}

func TestDockerExecutorForceStopsAndControllerRemovesOrphan(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-002", Suite: "integration", Tags: []string{"integration", "workload", "docker", "recovery"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	nodeID := fmt.Sprintf("stb402-orphan-%d", time.Now().UnixNano())
	executor := newDockerExecutor(t, nodeID)
	prepared, err := executor.Prepare(ctx, execution.Request{
		WorkloadID: "work.docker.orphan",
		Config:     dockerConfig(t, []string{"sh", "-c", "trap '' TERM; while true; do sleep 1; done"}),
	})
	require.NoError(t, err)
	instance, err := executor.Start(ctx, prepared)
	require.NoError(t, err)
	cleanupDockerInstance(t, executor, instance)

	started := time.Now()
	require.NoError(t, executor.Stop(ctx, instance))
	stopped := waitForDockerStopped(t, executor, prepared.WorkloadID)
	require.Less(t, time.Since(started), 8*time.Second)
	require.NotNil(t, stopped.ExitCode)
	require.NotZero(t, *stopped.ExitCode)

	restarted, err := executor.Start(ctx, prepared)
	require.NoError(t, err)
	require.True(t, restarted.Running)
	adapter := newControllerDockerExecutor(t, nodeID)
	service := workloadcontroller.New(t.TempDir()+"/ardents.db", adapter)
	require.NoError(t, service.Load())
	_, err = executor.Inspect(ctx, prepared.WorkloadID)
	require.ErrorContains(t, err, "not found")
}

func TestDockerExecutorEnforcesSecurityAndResourceConfiguration(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-004", Suite: "integration", Tags: []string{"integration", "workload", "docker", "security"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	executor := newDockerExecutor(t, fmt.Sprintf("stb403-bounds-%d", time.Now().UnixNano()))
	config := dockerConfigWith(t, map[string]any{
		"command": []string{"sh", "-c", "while true; do :; done"}, "env": map[string]string{"MODE": "acceptance"},
		"resources": map[string]int64{"memory_bytes": 64 * 1024 * 1024, "nano_cpus": 500_000_000, "pids": 16, "tmpfs_bytes": 1024 * 1024},
	})
	prepared, err := executor.Prepare(ctx, execution.Request{WorkloadID: "work.security.bounds", Config: config})
	require.NoError(t, err)
	instance, err := executor.Start(ctx, prepared)
	require.NoError(t, err)
	cleanupDockerInstance(t, executor, instance)

	engine, err := client.New(client.FromEnv)
	require.NoError(t, err)
	inspected, err := engine.ContainerInspect(ctx, instance.RuntimeID, client.ContainerInspectOptions{})
	require.NoError(t, err)
	require.Equal(t, "65534:65534", inspected.Container.Config.User)
	require.Contains(t, inspected.Container.Config.Env, "MODE=acceptance")
	require.Contains(t, inspected.Container.Config.Env, fmt.Sprintf("ARDENTS_WORKLOAD_GENERATION=%d", instance.Generation))
	require.True(t, inspected.Container.Config.NetworkDisabled)
	require.Equal(t, "none", string(inspected.Container.HostConfig.NetworkMode))
	require.Equal(t, "runc", inspected.Container.HostConfig.Runtime)
	require.True(t, inspected.Container.HostConfig.ReadonlyRootfs)
	require.False(t, inspected.Container.HostConfig.Privileged)
	require.Equal(t, []string{"ALL"}, inspected.Container.HostConfig.CapDrop)
	require.Contains(t, inspected.Container.HostConfig.SecurityOpt, "no-new-privileges:true")
	require.Empty(t, inspected.Container.HostConfig.Binds)
	require.Empty(t, inspected.Container.HostConfig.Devices)
	require.Equal(t, "local", inspected.Container.HostConfig.LogConfig.Type)
	require.Equal(t, "10m", inspected.Container.HostConfig.LogConfig.Config["max-size"])
	require.Equal(t, "2", inspected.Container.HostConfig.LogConfig.Config["max-file"])
	require.EqualValues(t, 64*1024*1024, inspected.Container.HostConfig.Memory)
	require.Equal(t, inspected.Container.HostConfig.Memory, inspected.Container.HostConfig.MemorySwap)
	require.EqualValues(t, 500_000_000, inspected.Container.HostConfig.NanoCPUs)
	require.EqualValues(t, 16, *inspected.Container.HostConfig.PidsLimit)
	require.Contains(t, inspected.Container.HostConfig.Tmpfs["/tmp"], "size=1048576")
	time.Sleep(1200 * time.Millisecond)
	statsResult, err := engine.ContainerStats(ctx, instance.RuntimeID, client.ContainerStatsOptions{})
	require.NoError(t, err)
	defer func() { require.NoError(t, statsResult.Body.Close()) }()
	var stats containerapi.StatsResponse
	require.NoError(t, json.NewDecoder(statsResult.Body).Decode(&stats))
	require.Positive(t, stats.CPUStats.ThrottlingData.Periods)
	require.Positive(t, stats.CPUStats.ThrottlingData.ThrottledPeriods)
}

//goland:noinspection ALL
func TestDockerExecutorPublishesOnlyAdmittedIngressOnInternalNetwork(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-004", Suite: "integration", Tags: []string{"integration", "workload", "docker", "security"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	host := dockerEngineIPv4(t)
	const port = 19090
	executor := dockerExecutorWith(t, workloaddocker.ExecutorConfig{
		NodeID: "stb405-ingress", Runtime: "runc", AllowedRegistries: []string{"docker.io"},
		AllowedIngressHosts: []string{host}, IngressBindAddress: "0.0.0.0", IngressProxyImage: dockerProxyImage(t),
	})
	prepared, err := executor.Prepare(ctx, execution.Request{
		WorkloadID: "work.ingress.echo",
		Config: dockerConfig(t, []string{"sh", "-c", fmt.Sprintf(
			"mkdir -p /tmp/www && printf ready >/tmp/www/index.html && exec httpd -f -p %d -h /tmp/www", port)}),
		Ingress: []execution.IngressRequest{{Mode: "NetworkPublished",
			Endpoints:      []string{"tcp://" + net.JoinHostPort(host, fmt.Sprint(port))},
			ProbeEndpoints: []string{fmt.Sprintf("tcp://127.0.0.1:%d", port)}}},
	})
	require.NoError(t, err)
	instance, err := executor.Start(ctx, prepared)
	require.NoError(t, err)
	cleanupDockerInstance(t, executor, instance)

	engine, err := client.New(client.FromEnv)
	require.NoError(t, err)
	inspected, err := engine.ContainerInspect(ctx, instance.RuntimeID, client.ContainerInspectOptions{})
	require.NoError(t, err)
	require.False(t, inspected.Container.Config.NetworkDisabled)
	require.NotEqual(t, "none", string(inspected.Container.HostConfig.NetworkMode))
	require.Empty(t, inspected.Container.HostConfig.PortBindings)
	network, err := engine.NetworkInspect(ctx, string(inspected.Container.HostConfig.NetworkMode), client.NetworkInspectOptions{})
	require.NoError(t, err)
	require.True(t, network.Network.Internal)
	filters := client.Filters{}
	filters = filters.Add("label", "io.ardents.ingress-proxy=true")
	filters = filters.Add("label", "io.ardents.workload=work.ingress.echo")
	proxies, err := engine.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: filters})
	require.NoError(t, err)
	require.Len(t, proxies.Items, 1)
	proxy, err := engine.ContainerInspect(ctx, proxies.Items[0].ID, client.ContainerInspectOptions{})
	require.NoError(t, err)
	require.Truef(t, proxy.Container.State.Running, "proxy state=%s error=%s", proxy.Container.State.Status, proxy.Container.State.Error)
	require.Len(t, proxy.Container.HostConfig.PortBindings, 1)
	proxyNetwork, err := engine.NetworkInspect(ctx, string(proxy.Container.HostConfig.NetworkMode), client.NetworkInspectOptions{})
	require.NoError(t, err)
	require.False(t, proxyNetwork.Network.Internal)

	httpClient := &http.Client{Timeout: time.Second}
	endpoint := "http://" + net.JoinHostPort(host, fmt.Sprint(port)) + "/"
	require.Eventually(t, func() bool {
		return dockerIngressReady(httpClient, endpoint)
	}, 10*time.Second, 100*time.Millisecond)

	_, err = engine.ContainerStop(ctx, proxy.Container.ID, client.ContainerStopOptions{Timeout: new(1)})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return !dockerIngressReady(httpClient, endpoint) }, 5*time.Second, 100*time.Millisecond)

	incompatibleExecutor := dockerExecutorWith(t, workloaddocker.ExecutorConfig{
		NodeID: "stb405-ingress", Runtime: "runc", AllowedRegistries: []string{"docker.io"},
		AllowedIngressHosts: []string{host}, IngressBindAddress: "0.0.0.0", IngressProxyImage: dockerImage(t),
	})
	err = incompatibleExecutor.ReconcileAncillary(ctx, []execution.Instance{instance})
	require.ErrorContains(t, err, "proxy protocol is incompatible")
	proxies, err = engine.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: filters})
	require.NoError(t, err)
	require.Empty(t, proxies.Items, "incompatible recovery must close the managed ingress proxy")

	recoveredExecutor := dockerExecutorWith(t, workloaddocker.ExecutorConfig{
		NodeID: "stb405-ingress", Runtime: "runc", AllowedRegistries: []string{"docker.io"},
		AllowedIngressHosts: []string{host}, IngressBindAddress: "0.0.0.0", IngressProxyImage: dockerProxyImage(t),
	})
	require.NoError(t, recoveredExecutor.ReconcileAncillary(ctx, []execution.Instance{instance}))
	require.Eventually(t, func() bool { return dockerIngressReady(httpClient, endpoint) }, 10*time.Second, 100*time.Millisecond)

	require.NoError(t, recoveredExecutor.ReconcileAncillary(ctx, nil))
	proxies, err = engine.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: filters})
	require.NoError(t, err)
	require.Empty(t, proxies.Items, "orphan ingress proxy must be removed when no current workload generation exists")
}

func dockerIngressReady(client *http.Client, endpoint string) bool {
	response, err := client.Get(endpoint)
	if err != nil {
		return false
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	return readErr == nil && closeErr == nil && response.StatusCode == http.StatusOK && string(body) == "ready"
}

func dockerEngineIPv4(t *testing.T) string {
	t.Helper()
	addresses, err := net.LookupIP("engine")
	require.NoError(t, err)
	for _, address := range addresses {
		if ipv4 := address.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	t.Fatal("Docker Engine fixture has no IPv4 address")
	return ""
}

func TestDockerExecutorDeniesUnsafeIntentAndRuntimeFallback(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-004", Suite: "integration", Tags: []string{"integration", "workload", "docker", "security"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	executor := newDockerExecutor(t, fmt.Sprintf("stb403-deny-%d", time.Now().UnixNano()))
	image := dockerImage(t)
	unsafe := []struct {
		name, config, reason, hidden string
	}{
		{"mount", fmt.Sprintf(`{"image":%q,"user":"65534","mounts":["/:/host"]}`, image), "unknown field", ""},
		{"secret", fmt.Sprintf(`{"image":%q,"user":"65534","env":{"API_TOKEN":"hidden-value"}}`, image), "cannot contain secret", "hidden-value"},
	}
	for _, tt := range unsafe {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executor.Prepare(ctx, execution.Request{WorkloadID: "work.deny." + tt.name, Config: tt.config})
			require.ErrorContains(t, err, tt.reason)
			if tt.hidden != "" {
				require.NotContains(t, err.Error(), tt.hidden)
			}
		})
	}

	deniedRegistry := dockerExecutorWith(t, workloaddocker.ExecutorConfig{
		NodeID: "stb403-registry", Runtime: "runc", AllowedRegistries: []string{"example.invalid"},
	})
	_, err := deniedRegistry.Prepare(ctx, execution.Request{WorkloadID: "work.registry", Config: dockerConfig(t, []string{"true"})})
	require.ErrorContains(t, err, "registry docker.io is not allowed")

	_, err = executor.Prepare(ctx, execution.Request{WorkloadID: "work.policy", PolicyRef: "trusted-other", Config: dockerConfig(t, []string{"true"})})
	require.ErrorContains(t, err, "policy reference is not allowed")

	noFallback := dockerExecutorWith(t, workloaddocker.ExecutorConfig{
		NodeID: "stb403-runtime", UntrustedRuntime: "missing-runsc", AllowedRegistries: []string{"docker.io"},
	})
	prepared, err := noFallback.Prepare(ctx, execution.Request{WorkloadID: "work.untrusted", Config: dockerConfig(t, []string{"true"})})
	require.NoError(t, err)
	_, err = noFallback.Start(ctx, prepared)
	require.ErrorContains(t, err, "create workload container")
	managed, listErr := noFallback.Managed(ctx)
	require.NoError(t, listErr)
	require.Empty(t, managed)
}

func TestDockerExecutorDeniesFilesystemNetworkProcessAndMemoryPressure(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-004", Suite: "integration", Tags: []string{"integration", "workload", "docker", "security"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	executor := newDockerExecutor(t, fmt.Sprintf("stb403-pressure-%d", time.Now().UnixNano()))
	tests := []struct {
		id      string
		command string
	}{
		{"readonly", "touch /etc/ardents-escape"},
		{"network", "wget -T 2 -O /tmp/out http://1.1.1.1"},
		{"tmpfs", "dd if=/dev/zero of=/tmp/fill bs=1048576 count=2"},
		{"pids", "set -e; i=0; while [ $i -lt 100 ]; do sleep 5 & i=$((i+1)); done; wait"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			config := dockerConfigWith(t, map[string]any{
				"command":   []string{"sh", "-c", tt.command},
				"resources": map[string]int64{"memory_bytes": 64 * 1024 * 1024, "pids": 16, "tmpfs_bytes": 1024 * 1024},
			})
			assertDockerCommandFails(t, ctx, executor, "work.pressure."+tt.id, config, false)
		})
	}
	oomConfig := dockerConfigWith(t, map[string]any{
		"command":   []string{"sh", "-c", `awk 'BEGIN { for (i=0;;i++) a[i]=sprintf("%0100000d", i) }'`},
		"resources": map[string]int64{"memory_bytes": 32 * 1024 * 1024, "pids": 16},
	})
	assertDockerCommandFails(t, ctx, executor, "work.pressure.oom", oomConfig, true)
}

func TestDockerExecutorControllerSurfacesOOMAndBlocksAutomaticRestart(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-004", Suite: "integration", Tags: []string{"integration", "workload", "docker", "security"}, Speed: "default", Environment: "linux-container"})
	requireDockerFixture(t)
	beginDockerScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	nodeID := fmt.Sprintf("stb403-oom-%d", time.Now().UnixNano())
	adapter := newControllerDockerExecutor(t, nodeID)
	observer := newDockerExecutor(t, nodeID)
	service := workloadcontroller.New(t.TempDir()+"/ardents.db", adapter)
	require.NoError(t, service.Load())
	config := dockerConfigWith(t, map[string]any{
		"command":   []string{"sh", "-c", `awk 'BEGIN { for (i=0;;i++) a[i]=sprintf("%0100000d", i) }'`},
		"resources": map[string]int64{"memory_bytes": 32 * 1024 * 1024, "pids": 16},
	})
	require.NoError(t, service.Register(workloadregistry.Spec{
		ID: "work.controller.oom", Kind: "worker", Owner: "node", Config: config,
		Desired: workloadregistry.DesiredRunning, PolicyRef: "trusted",
	}))
	require.NoError(t, service.Reconcile(ctx))
	started, ok := service.Get("work.controller.oom")
	require.True(t, ok)
	cleanupDockerInstance(t, observer, started.Instance)
	stopped := waitForDockerStopped(t, observer, "work.controller.oom")
	require.True(t, stopped.OOMKilled)

	changed, err := service.RefreshObserved(ctx)
	require.NoError(t, err)
	require.True(t, changed)
	failed, ok := service.Get("work.controller.oom")
	require.True(t, ok)
	require.Equal(t, workloadcontroller.ObservedFailed, failed.Observed)
	require.Contains(t, failed.Reason, "memory limit")
	require.True(t, failed.NeedsOperatorAction)
	require.Equal(t, 1, failed.RestartCount)

	require.NoError(t, service.Reconcile(ctx))
	blocked, ok := service.Get("work.controller.oom")
	require.True(t, ok)
	require.Equal(t, stopped.RuntimeID, blocked.Instance.RuntimeID)
	require.Equal(t, workloadcontroller.ObservedFailed, blocked.Observed)
}

func requireDockerFixture(t *testing.T) {
	t.Helper()
	if os.Getenv("DOCKER_HOST") == "" || os.Getenv("ARDENTS_TEST_IMAGE_FILE") == "" || os.Getenv("ARDENTS_TEST_PROXY_IMAGE_FILE") == "" {
		t.Skip("isolated Docker Engine fixture is provided by tests/run-workload-docker.ps1")
	}
}

func beginDockerScenario(t *testing.T) {
	t.Helper()
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "workload", ScenarioID: "WKI-002",
		Suite: "integration", Tags: []string{"integration", "workload", "docker"},
		Speed: "slow", Environment: "linux-dind",
	})
}

func newDockerExecutor(t *testing.T, nodeID string) *workloaddocker.Executor {
	t.Helper()
	return dockerExecutorWith(t, workloaddocker.ExecutorConfig{
		NodeID: nodeID, Runtime: "runc", AllowedRegistries: []string{"docker.io"},
		AllowedPolicyRefs: []string{"trusted"}, StopTimeout: 2 * time.Second,
	})
}

func dockerExecutorWith(t *testing.T, config workloaddocker.ExecutorConfig) *workloaddocker.Executor {
	t.Helper()
	config.AllowInsecureRemote = true
	executor, err := workloaddocker.NewExecutor(config)
	require.NoError(t, err)
	return executor
}

func newControllerDockerExecutor(t *testing.T, nodeID string) workloadcontroller.Executor {
	t.Helper()
	executor, err := workloaddocker.NewExecutor(workloaddocker.ExecutorConfig{
		NodeID: nodeID, Runtime: "runc", AllowedRegistries: []string{"docker.io"},
		AllowedPolicyRefs: []string{"trusted"}, AllowInsecureRemote: true, StopTimeout: 2 * time.Second,
	})
	require.NoError(t, err)
	return executor
}

func dockerConfig(t *testing.T, command []string) string {
	t.Helper()
	return dockerConfigWith(t, map[string]any{"command": command})
}

func dockerConfigWith(t *testing.T, fields map[string]any) string {
	t.Helper()
	spec := map[string]any{"image": dockerImage(t), "user": "65534:65534"}
	for key, value := range fields {
		spec[key] = value
	}
	encoded, err := json.Marshal(spec)
	require.NoError(t, err)
	return string(encoded)
}

func dockerImage(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(os.Getenv("ARDENTS_TEST_IMAGE_FILE"))
	require.NoError(t, err)
	return strings.TrimSpace(string(raw))
}

func dockerProxyImage(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(os.Getenv("ARDENTS_TEST_PROXY_IMAGE_FILE"))
	require.NoError(t, err)
	return strings.TrimSpace(string(raw))
}

func assertDockerCommandFails(
	t *testing.T,
	ctx context.Context,
	executor *workloaddocker.Executor,
	workloadID, config string,
	expectOOM bool,
) {
	t.Helper()
	prepared, err := executor.Prepare(ctx, execution.Request{WorkloadID: workloadID, Config: config})
	require.NoError(t, err)
	instance, err := executor.Start(ctx, prepared)
	require.NoError(t, err)
	cleanupDockerInstance(t, executor, instance)
	stopped := waitForDockerStopped(t, executor, workloadID)
	require.NotNil(t, stopped.ExitCode)
	require.NotZero(t, *stopped.ExitCode)
	require.Equal(t, expectOOM, stopped.OOMKilled)
	if expectOOM {
		require.Equal(t, "oom_killed", stopped.Reason)
	}
}

func waitForDockerStopped(t *testing.T, executor *workloaddocker.Executor, workloadID string) execution.Instance {
	t.Helper()
	var current execution.Instance
	require.Eventually(t, func() bool {
		var err error
		current, err = executor.Inspect(context.Background(), workloadID)
		return err == nil && !current.Running
	}, 10*time.Second, 100*time.Millisecond)
	return current
}

func cleanupDockerInstance(t *testing.T, executor *workloaddocker.Executor, instance execution.Instance) {
	t.Helper()
	t.Cleanup(func() {
		current, err := executor.Inspect(context.Background(), instance.WorkloadID)
		if err != nil {
			return
		}
		if current.Running {
			_ = executor.Stop(context.Background(), current)
			current = waitForDockerStopped(t, executor, instance.WorkloadID)
		}
		_ = executor.Remove(context.Background(), current)
	})
}
