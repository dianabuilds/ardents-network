package observability

import (
	dataapi "ardents/internal/data/api"
	diagapi "ardents/internal/diagnostics/api"
	hostingapi "ardents/internal/hosting/api"
	nodeapi "ardents/internal/node/api"
	workloadapi "ardents/internal/workload/api"

	"github.com/prometheus/client_golang/prometheus"
)

type Source interface {
	GetNodeRuntime() nodeapi.NodeRuntimeSnapshot
	GetNetworkStatus() nodeapi.NetworkStatusSnapshot
	ListPeers() []nodeapi.PeerSnapshot
	DiagnosticsSnapshot() diagapi.DiagSnapshot
	ListWorkloads() ([]workloadapi.WorkloadStatusSnapshot, error)
	ListHostedServices() ([]hostingapi.HostedServiceSnapshot, error)
	DataInventory() dataapi.DataInventorySnapshot
	ListTransfers() []dataapi.TransferSnapshot
}

type Collector struct {
	source Source
	desc   descriptors
}

func NewCollector(source Source) *Collector {
	return &Collector{source: source, desc: newDescriptors()}
}

func (c *Collector) Describe(out chan<- *prometheus.Desc) {
	c.desc.describe(out)
}

func (c *Collector) Collect(out chan<- prometheus.Metric) {
	runtime := c.source.GetNodeRuntime()
	network := c.source.GetNetworkStatus()
	diagnostics := c.source.DiagnosticsSnapshot()
	c.collectNode(out, runtime)
	c.collectNetwork(out, network)
	c.collectPeers(out, c.source.ListPeers())
	c.collectDiagnostics(out, diagnostics)
	c.collectWorkloads(out)
	c.collectServices(out)
	c.collectData(out)
}

func gauge(out chan<- prometheus.Metric, desc *prometheus.Desc, value float64, labels ...string) {
	out <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
}

func counter(out chan<- prometheus.Metric, desc *prometheus.Desc, value float64, labels ...string) {
	out <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, value, labels...)
}

func (c *Collector) collectNode(out chan<- prometheus.Metric, runtime nodeapi.NodeRuntimeSnapshot) {
	gauge(out, c.desc.nodeState, 1, lifecycleState(runtime.Node.State))
	gauge(out, c.desc.nodeReady, boolValue(runtime.Node.Ready))
	gauge(out, c.desc.healthState, 1, healthState(runtime.Health.State))
}

func (c *Collector) collectNetwork(out chan<- prometheus.Metric, snapshot nodeapi.NetworkStatusSnapshot) {
	for _, protocol := range protocols(snapshot.ActiveCapabilities) {
		gauge(out, c.desc.wakuProtocol, 1, protocol)
	}
	values := []struct {
		reason string
		value  uint64
	}{
		{"rate_limited", snapshot.RateLimitedOperations},
		{"backpressured", snapshot.BackpressuredOperations},
		{"oversized", snapshot.OversizedMessages},
	}
	for _, item := range values {
		counter(out, c.desc.networkRejections, float64(item.value), item.reason)
	}
}

func (c *Collector) collectPeers(out chan<- prometheus.Metric, peers []nodeapi.PeerSnapshot) {
	counts := map[[2]string]int{}
	for _, peer := range peers {
		key := [2]string{peerState(peer.State), trustState(peer.Trust)}
		counts[key]++
	}
	for key, count := range counts {
		gauge(out, c.desc.peers, float64(count), key[0], key[1])
	}
}

func (c *Collector) collectDiagnostics(out chan<- prometheus.Metric, snapshot diagapi.DiagSnapshot) {
	for _, subsystem := range snapshot.Health.Subsystems {
		gauge(out, c.desc.subsystemHealth, 1, domain(subsystem.Domain), healthState(subsystem.State))
	}
	operationCounts := map[[2]string]int{}
	for _, operation := range snapshot.PendingOperations {
		operationCounts[[2]string{domain(operation.Domain), operationState(operation.State)}]++
	}
	for key, count := range operationCounts {
		gauge(out, c.desc.pendingOperations, float64(count), key[0], key[1])
	}
	c.collectEventWindow(out, snapshot.RecentEvents)
}

func (c *Collector) collectEventWindow(out chan<- prometheus.Metric, events []diagapi.EventEnvelope) {
	privacy := map[[2]string]int{}
	repairs := map[string]int{}
	denials := map[string]int{}
	messages := map[string]int{}
	for _, event := range events {
		classifyEvent(event, privacy, repairs, denials, messages)
	}
	for key, count := range privacy {
		gauge(out, c.desc.privacyFailures, float64(count), key[0], key[1])
	}
	for outcome, count := range repairs {
		gauge(out, c.desc.repairs, float64(count), outcome)
	}
	for action, count := range denials {
		gauge(out, c.desc.policyDenials, float64(count), action)
	}
	for category, count := range messages {
		gauge(out, c.desc.messageFailures, float64(count), category)
	}
}

func (c *Collector) collectWorkloads(out chan<- prometheus.Metric) {
	items, err := c.source.ListWorkloads()
	if err != nil {
		gauge(out, c.desc.collectionErrors, 1, "workload")
		return
	}
	states := map[string]int{}
	var memory, cpus, pids, restarts, oom float64
	for _, item := range items {
		states[workloadState(item.Observed)]++
		memory += float64(item.Instance.MemoryLimitBytes)
		cpus += float64(item.Instance.NanoCPUs)
		pids += float64(item.Instance.PIDsLimit)
		restarts += float64(item.Instance.Restarts)
		if item.Instance.OOMKilled {
			oom++
		}
	}
	for state, count := range states {
		gauge(out, c.desc.workloads, float64(count), state)
	}
	gauge(out, c.desc.workloadLimits, memory, "memory_bytes")
	gauge(out, c.desc.workloadLimits, cpus, "nano_cpus")
	gauge(out, c.desc.workloadLimits, pids, "pids")
	gauge(out, c.desc.workloadRestarts, restarts)
	gauge(out, c.desc.workloadOOM, oom)
}

func (c *Collector) collectServices(out chan<- prometheus.Metric) {
	items, err := c.source.ListHostedServices()
	if err != nil {
		gauge(out, c.desc.collectionErrors, 1, "hosting")
		return
	}
	counts := map[string]int{}
	for _, item := range items {
		counts[readinessState(item.Readiness, item.Ready)]++
	}
	for state, count := range counts {
		gauge(out, c.desc.hostedServices, float64(count), state)
	}
}

func (c *Collector) collectData(out chan<- prometheus.Metric) {
	inventory := c.source.DataInventory()
	items := map[string]int{"objects": inventory.Objects, "manifests": inventory.Manifests, "blobs": inventory.Blobs,
		"local_blobs": inventory.LocalBlobs, "remote_blobs": inventory.RemoteBlobs, "pinned": inventory.Pinned,
		"expired": inventory.Expired, "deleted": inventory.Deleted}
	for kind, count := range items {
		gauge(out, c.desc.storageItems, float64(count), kind)
	}
	gauge(out, c.desc.storageBytes, float64(inventory.LocalBytes), "local")
	gauge(out, c.desc.storageBytes, float64(inventory.RelayBytes), "relay")
	transfers := map[[2]string]int{}
	for _, transfer := range c.source.ListTransfers() {
		transfers[[2]string{transferState(transfer.State), direction(transfer.Direction)}]++
	}
	for key, count := range transfers {
		gauge(out, c.desc.transfers, float64(count), key[0], key[1])
	}
}

func boolValue(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
