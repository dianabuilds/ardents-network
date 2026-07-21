// Package auth owns local-protocol authentication, method authorization, and audit context.
// It does not own product policy.
package auth

import (
	"sort"

	identityapi "ardents/internal/identity"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

type Rule struct {
	Action string
	Domain string
	Access identityapi.Access
}

var procedureAccess = map[string]Rule{
	ardentsv1connect.NodeServiceStartNodeProcedure:                          action("node.start", "node", identityapi.AccessWrite),
	ardentsv1connect.NodeServiceStopNodeProcedure:                           action("node.stop", "node", identityapi.AccessWrite),
	ardentsv1connect.NodeServiceGetNodeStatusProcedure:                      action("node.status", "node", identityapi.AccessRead),
	ardentsv1connect.NodeServiceGetNodeCapabilitiesProcedure:                action("node.capabilities", "node", identityapi.AccessRead),
	ardentsv1connect.NodeServiceGetNodeRuntimeProcedure:                     action("node.runtime", "node", identityapi.AccessRead),
	ardentsv1connect.ConfigurationServiceGetEffectiveConfigurationProcedure: action("config.effective", "config", identityapi.AccessRead),
	ardentsv1connect.ConfigurationServiceReloadConfigurationProcedure:       action("config.reload", "config", identityapi.AccessWrite),
	ardentsv1connect.NodeServiceStreamNodeEventsProcedure:                   action("node.events", "node", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceGetNetworkStatusProcedure:                action("transport.network_status", "transport", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceGetDiscoveryStatusProcedure:              action("discovery.status", "discovery", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceGetLocalPresenceProcedure:                action("discovery.local_presence", "discovery", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceListPeersProcedure:                       action("discovery.peers", "discovery", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceListRouteCandidatesProcedure:             action("transport.route_candidates", "transport", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceResolveRecordProcedure:                   action("discovery.resolve_record", "discovery", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceResolveServiceProcedure:                  action("discovery.resolve_service", "discovery", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceListRecordsProcedure:                     action("discovery.list_records", "discovery", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceImportRecordProcedure:                    action("discovery.import", "discovery", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure:               action("workload.register", "workload", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceStartWorkloadProcedure:                  action("workload.start", "workload", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceStopWorkloadProcedure:                   action("workload.stop", "workload", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceRestartWorkloadProcedure:                action("workload.restart", "workload", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure:              action("workload.status", "workload", identityapi.AccessRead),
	ardentsv1connect.WorkloadServiceListWorkloadsProcedure:                  action("workload.list", "workload", identityapi.AccessRead),
	ardentsv1connect.WorkloadServiceGetHostedServiceProcedure:               action("workload.hosted_service", "workload", identityapi.AccessRead),
	ardentsv1connect.WorkloadServiceListHostedServicesProcedure:             action("workload.hosted_services", "workload", identityapi.AccessRead),
	ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure:    action("workload.service_publication", "workload", identityapi.AccessRead),
	ardentsv1connect.ContentServicePublishObjectProcedure:                   action("data.publish_object", "data", identityapi.AccessWrite),
	ardentsv1connect.ContentServiceGetObjectProcedure:                       action("data.get_object", "data", identityapi.AccessRead),
	ardentsv1connect.ContentServiceListObjectsProcedure:                     action("data.list_objects", "data", identityapi.AccessRead),
	ardentsv1connect.ContentServicePublishBlobProcedure:                     action("data.publish_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.TransferServiceFetchBlobProcedure:                      action("data.fetch_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.ContentServiceGetBlobProcedure:                         action("data.get_blob", "data", identityapi.AccessRead),
	ardentsv1connect.ContentServiceListBlobsProcedure:                       action("data.list_blobs", "data", identityapi.AccessRead),
	ardentsv1connect.TransferServiceListBlobSourcesProcedure:                action("data.blob_sources", "data", identityapi.AccessRead),
	ardentsv1connect.TransferServiceGetTransferProcedure:                    action("data.get_transfer", "data", identityapi.AccessRead),
	ardentsv1connect.TransferServiceListTransfersProcedure:                  action("data.list_transfers", "data", identityapi.AccessRead),
	ardentsv1connect.ContentServicePublishManifestProcedure:                 action("data.publish_manifest", "data", identityapi.AccessWrite),
	ardentsv1connect.ContentServiceGetManifestProcedure:                     action("data.get_manifest", "data", identityapi.AccessRead),
	ardentsv1connect.ContentServiceListManifestsProcedure:                   action("data.list_manifests", "data", identityapi.AccessRead),
	ardentsv1connect.RetentionServiceRetainBlobProcedure:                    action("data.retain_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.RetentionServicePinBlobProcedure:                       action("data.pin_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.RetentionServiceDropBlobProcedure:                      action("data.drop_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.ContentServiceGetDataInventoryProcedure:                action("data.inventory", "data", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceGetDiagnosticsProcedure:              action("diagnostics.snapshot", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceGetPendingOperationsProcedure:        action("diagnostics.pending_operations", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceGetHealthSummaryProcedure:            action("diagnostics.health_summary", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceExplainFailureProcedure:              action("diagnostics.explain_failure", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure:            action("diagnostics.recent_events", "diagnostics", identityapi.AccessRead),
}

func action(name, domain string, access identityapi.Access) Rule {
	return Rule{Action: name, Domain: domain, Access: access}
}

func RuleForProcedure(procedure string) (Rule, bool) {
	rule, ok := procedureAccess[procedure]
	return rule, ok
}

func OperatorActions() []string {
	actions := make([]string, 0, len(procedureAccess))
	for _, rule := range procedureAccess {
		actions = append(actions, rule.Action)
	}
	sort.Strings(actions)
	return actions
}
