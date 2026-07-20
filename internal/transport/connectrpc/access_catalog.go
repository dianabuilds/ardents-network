package connectrpc

import (
	"sort"

	identityapi "ardents/internal/identity/api"
	"ardents/proto/ardents/v1/ardentsv1connect"
)

type accessRule struct {
	Action string
	Domain string
	Access identityapi.Access
}

var procedureAccess = map[string]accessRule{
	ardentsv1connect.ArdentsServiceStartNodeProcedure:                   action("node.start", "node", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceStopNodeProcedure:                    action("node.stop", "node", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceGetNodeStatusProcedure:               action("node.status", "node", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetNodeCapabilitiesProcedure:         action("node.capabilities", "node", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetNodeRuntimeProcedure:              action("node.runtime", "node", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetEffectiveConfigurationProcedure:   action("config.effective", "config", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceReloadConfigurationProcedure:         action("config.reload", "config", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceStreamNodeEventsProcedure:            action("node.events", "node", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetNetworkStatusProcedure:            action("transport.network_status", "transport", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetDiscoveryStatusProcedure:          action("discovery.status", "discovery", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetLocalPresenceProcedure:            action("discovery.local_presence", "discovery", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListPeersProcedure:                   action("discovery.peers", "discovery", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListRouteCandidatesProcedure:         action("transport.route_candidates", "transport", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceResolveRecordProcedure:               action("discovery.resolve_record", "discovery", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceResolveServiceProcedure:              action("discovery.resolve_service", "discovery", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListRecordsProcedure:                 action("discovery.list_records", "discovery", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceImportRecordProcedure:                action("discovery.import", "discovery", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceRegisterWorkloadProcedure:            action("workload.register", "workload", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceStartWorkloadProcedure:               action("workload.start", "workload", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceStopWorkloadProcedure:                action("workload.stop", "workload", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceRestartWorkloadProcedure:             action("workload.restart", "workload", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceGetWorkloadStatusProcedure:           action("workload.status", "workload", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListWorkloadsProcedure:               action("workload.list", "workload", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetHostedServiceProcedure:            action("workload.hosted_service", "workload", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListHostedServicesProcedure:          action("workload.hosted_services", "workload", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetServicePublicationStatusProcedure: action("workload.service_publication", "workload", identityapi.AccessRead),
	ardentsv1connect.ArdentsServicePublishObjectProcedure:               action("data.publish_object", "data", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceGetObjectProcedure:                   action("data.get_object", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListObjectsProcedure:                 action("data.list_objects", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServicePublishBlobProcedure:                 action("data.publish_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceFetchBlobProcedure:                   action("data.fetch_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceGetBlobProcedure:                     action("data.get_blob", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListBlobsProcedure:                   action("data.list_blobs", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListBlobSourcesProcedure:             action("data.blob_sources", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetTransferProcedure:                 action("data.get_transfer", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListTransfersProcedure:               action("data.list_transfers", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServicePublishManifestProcedure:             action("data.publish_manifest", "data", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceGetManifestProcedure:                 action("data.get_manifest", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListManifestsProcedure:               action("data.list_manifests", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceRetainBlobProcedure:                  action("data.retain_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServicePinBlobProcedure:                     action("data.pin_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceDropBlobProcedure:                    action("data.drop_blob", "data", identityapi.AccessWrite),
	ardentsv1connect.ArdentsServiceGetDataInventoryProcedure:            action("data.inventory", "data", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetDiagnosticsProcedure:              action("diagnostics.snapshot", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetPendingOperationsProcedure:        action("diagnostics.pending_operations", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceGetHealthSummaryProcedure:            action("diagnostics.health_summary", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceExplainFailureProcedure:              action("diagnostics.explain_failure", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.ArdentsServiceListRecentEventsProcedure:            action("diagnostics.recent_events", "diagnostics", identityapi.AccessRead),
}

func action(name, domain string, access identityapi.Access) accessRule {
	return accessRule{Action: name, Domain: domain, Access: access}
}

func OperatorActions() []string {
	actions := make([]string, 0, len(procedureAccess))
	for _, rule := range procedureAccess {
		actions = append(actions, rule.Action)
	}
	sort.Strings(actions)
	return actions
}
