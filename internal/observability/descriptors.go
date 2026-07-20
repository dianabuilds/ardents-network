package observability

import "github.com/prometheus/client_golang/prometheus"

type descriptors struct {
	nodeState, nodeReady, healthState, peers, wakuProtocol     *prometheus.Desc
	networkRejections, privacyFailures, messageFailures        *prometheus.Desc
	workloads, workloadLimits, workloadRestarts, workloadOOM   *prometheus.Desc
	hostedServices, storageItems, storageBytes, transfers      *prometheus.Desc
	repairs, policyDenials, pendingOperations, subsystemHealth *prometheus.Desc
	collectionErrors                                           *prometheus.Desc
}

func newDescriptors() descriptors {
	return descriptors{
		nodeState:         metric("node_lifecycle", "Current canonical lifecycle state.", "state"),
		nodeReady:         metric("node_ready", "Whether the node is canonically ready."),
		healthState:       metric("node_health", "Current canonical Diagnostics health state.", "state"),
		peers:             metric("peers", "Observed peers grouped by bounded state and trust.", "state", "trust"),
		wakuProtocol:      metric("waku_protocol_active", "Active canonical Waku protocol capabilities.", "protocol"),
		networkRejections: metric("network_rejections_total", "Cumulative network protection rejections.", "reason"),
		privacyFailures:   metric("privacy_failures_window", "Privacy failures in the bounded Diagnostics window.", "domain", "category"),
		messageFailures:   metric("message_failures_window", "Message failures in the bounded Diagnostics window.", "category"),
		workloads:         metric("workloads", "Workloads grouped by bounded observed state.", "state"),
		workloadLimits:    metric("workload_resource_limits", "Aggregate configured workload resource limits.", "resource"),
		workloadRestarts:  metric("workload_restarts", "Aggregate restart count reported by workload runtimes."),
		workloadOOM:       metric("workload_oom_killed", "Workloads whose latest runtime outcome is OOM killed."),
		hostedServices:    metric("hosted_services", "Hosted services grouped by readiness.", "readiness"),
		storageItems:      metric("storage_items", "Storage inventory grouped by bounded item kind.", "kind"),
		storageBytes:      metric("storage_bytes", "Storage bytes grouped by bounded retention class.", "class"),
		transfers:         metric("transfers", "Transfers grouped by bounded state and direction.", "state", "direction"),
		repairs:           metric("repairs_window", "Repair outcomes in the bounded Diagnostics window.", "outcome"),
		policyDenials:     metric("policy_denials_window", "Policy denials in the bounded Diagnostics window.", "action"),
		pendingOperations: metric("pending_operations", "Pending operations grouped by domain and state.", "domain", "state"),
		subsystemHealth:   metric("subsystem_health", "Diagnostics subsystem health grouped by bounded domain and state.", "domain", "state"),
		collectionErrors:  metric("collection_errors", "Snapshot collection errors by bounded domain.", "domain"),
	}
}

func metric(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName("ardents", "", name), help, labels, nil)
}

func (d descriptors) describe(out chan<- *prometheus.Desc) {
	items := []*prometheus.Desc{
		d.nodeState, d.nodeReady, d.healthState, d.peers, d.wakuProtocol,
		d.networkRejections, d.privacyFailures, d.messageFailures,
		d.workloads, d.workloadLimits, d.workloadRestarts, d.workloadOOM,
		d.hostedServices, d.storageItems, d.storageBytes, d.transfers,
		d.repairs, d.policyDenials, d.pendingOperations, d.subsystemHealth,
		d.collectionErrors,
	}
	for _, item := range items {
		out <- item
	}
}
