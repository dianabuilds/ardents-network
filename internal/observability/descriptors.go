package observability

import "github.com/prometheus/client_golang/prometheus"

type descriptors struct {
	nodeState, nodeReady, healthState, peers, wakuProtocol     *prometheus.Desc
	networkRejections, privacyFailures, messageFailures        *prometheus.Desc
	wakuStoreMessages, wakuStoreCapacity, wakuStoreFileBytes   *prometheus.Desc
	wakuStoreCapacityBytes                                     *prometheus.Desc
	wakuStoreUsageRatio                                        *prometheus.Desc
	workloads, workloadLimits, workloadRestarts, workloadOOM   *prometheus.Desc
	hostedServices, storageItems, storageBytes, transfers      *prometheus.Desc
	repairs, policyDenials, pendingOperations, subsystemHealth *prometheus.Desc
	authorityState, authorityPhase, authorityAuditOutbox       *prometheus.Desc
	authorityMembers, authorityChannels, authorityOperations   *prometheus.Desc
	collectionErrors                                           *prometheus.Desc
}

func newDescriptors() descriptors {
	return descriptors{
		nodeState:              metric("node_lifecycle", "Current canonical lifecycle state.", "state"),
		nodeReady:              metric("node_ready", "Whether the node is canonically ready."),
		healthState:            metric("node_health", "Current canonical Diagnostics health state.", "state"),
		peers:                  metric("peers", "Observed peers grouped by bounded state and trust.", "state", "trust"),
		wakuProtocol:           metric("waku_protocol_active", "Active canonical Waku transport features.", "protocol"),
		networkRejections:      metric("network_rejections_total", "Cumulative network protection rejections.", "reason"),
		wakuStoreMessages:      metric("waku_store_messages", "Current messages retained by the unauthenticated Waku Store."),
		wakuStoreCapacity:      metric("waku_store_capacity_messages", "Configured finite Waku Store message capacity."),
		wakuStoreCapacityBytes: metric("waku_store_capacity_bytes", "Configured hard Waku Store disk capacity in bytes."),
		wakuStoreFileBytes:     metric("waku_store_file_bytes", "Current Waku Store SQLite file size in bytes."),
		wakuStoreUsageRatio:    metric("waku_store_usage_ratio", "Greatest current Waku Store message, main database, or WAL capacity utilization ratio."),
		privacyFailures:        metric("privacy_failures_window", "Privacy failures in the bounded Diagnostics window.", "domain", "category"),
		messageFailures:        metric("message_failures_window", "Message failures in the bounded Diagnostics window.", "category"),
		workloads:              metric("workloads", "Workloads grouped by bounded observed state.", "state"),
		workloadLimits:         metric("workload_resource_limits", "Aggregate configured workload resource limits.", "resource"),
		workloadRestarts:       metric("workload_restarts", "Aggregate restart count reported by workload runtimes."),
		workloadOOM:            metric("workload_oom_killed", "Workloads whose latest runtime outcome is OOM killed."),
		hostedServices:         metric("hosted_services", "Hosted services grouped by readiness.", "readiness"),
		storageItems:           metric("storage_items", "Storage inventory grouped by bounded item kind.", "kind"),
		storageBytes:           metric("storage_bytes", "Storage bytes grouped by bounded retention class.", "class"),
		transfers:              metric("transfers", "Transfers grouped by bounded state and direction.", "state", "direction"),
		repairs:                metric("repairs_window", "Repair outcomes in the bounded Diagnostics window.", "outcome"),
		policyDenials:          metric("policy_denials_window", "Policy denials in the bounded Diagnostics window.", "action"),
		pendingOperations:      metric("pending_operations", "Pending operations grouped by domain and state.", "domain", "state"),
		subsystemHealth:        metric("subsystem_health", "Diagnostics subsystem health grouped by bounded domain and state.", "domain", "state"),
		authorityState:         metric("realm_authority_readiness", "Current Realm Authority readiness and bounded reason.", "state", "reason"),
		authorityPhase:         metric("realm_authority_phase", "Current Realm Authority lifecycle phase.", "phase"),
		authorityAuditOutbox:   metric("realm_authority_audit_outbox_depth", "Pending durable Realm Authority audit records."),
		authorityMembers:       metric("realm_authority_members", "Current Realm Authority member count."),
		authorityChannels:      metric("realm_authority_channels", "Current Realm Authority channel count."),
		authorityOperations:    metric("realm_authority_pending_operations", "Current pending Realm Authority operation count."),
		collectionErrors:       metric("collection_errors", "Snapshot collection errors by bounded domain.", "domain"),
	}
}

func metric(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName("ardents", "", name), help, labels, nil)
}

func (d descriptors) describe(out chan<- *prometheus.Desc) {
	items := []*prometheus.Desc{
		d.nodeState, d.nodeReady, d.healthState, d.peers, d.wakuProtocol,
		d.networkRejections, d.privacyFailures, d.messageFailures,
		d.wakuStoreMessages, d.wakuStoreCapacity, d.wakuStoreCapacityBytes, d.wakuStoreFileBytes, d.wakuStoreUsageRatio,
		d.workloads, d.workloadLimits, d.workloadRestarts, d.workloadOOM,
		d.hostedServices, d.storageItems, d.storageBytes, d.transfers,
		d.repairs, d.policyDenials, d.pendingOperations, d.subsystemHealth,
		d.authorityState, d.authorityPhase, d.authorityAuditOutbox,
		d.authorityMembers, d.authorityChannels, d.authorityOperations,
		d.collectionErrors,
	}
	for _, item := range items {
		out <- item
	}
}
