// Package auth owns the frozen Operator action catalogue and canonical request mapping.
package auth

import (
	"errors"
	"sort"

	identityapi "ardents/internal/identity"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

var ErrUnknownProcedure = errors.New("operator procedure is not registered")

type Rule struct {
	Action       string
	Domain       string
	ResourceKind string
	Access       identityapi.Access
}

var procedureAccess = map[string]Rule{
	ardentsv1connect.NodeServiceStartNodeProcedure:                          resourceAction("node.start", "node", "node", identityapi.AccessWrite),
	ardentsv1connect.NodeServiceStopNodeProcedure:                           resourceAction("node.stop", "node", "node", identityapi.AccessWrite),
	ardentsv1connect.NodeServiceGetNodeStatusProcedure:                      resourceAction("node.status", "node", "node", identityapi.AccessRead),
	ardentsv1connect.NodeServiceGetNodeFeaturesProcedure:                    resourceAction("node.features", "node", "node", identityapi.AccessRead),
	ardentsv1connect.NodeServiceGetNodeRuntimeProcedure:                     resourceAction("node.runtime", "node", "node", identityapi.AccessRead),
	ardentsv1connect.ConfigurationServiceGetEffectiveConfigurationProcedure: resourceAction("config.effective", "config", "configuration", identityapi.AccessRead),
	ardentsv1connect.ConfigurationServiceReloadConfigurationProcedure:       resourceAction("config.reload", "config", "configuration", identityapi.AccessWrite),
	ardentsv1connect.NodeServiceStreamNodeEventsProcedure:                   resourceAction("node.events", "node", "node", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceGetNetworkStatusProcedure:                resourceAction("transport.network_status", "transport", "network", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceGetDiscoveryStatusProcedure:              resourceAction("discovery.status", "discovery", "discovery-status", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceGetLocalPresenceProcedure:                resourceAction("discovery.local_presence", "discovery", "local-presence", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceListPeersProcedure:                       resourceAction("discovery.peers", "discovery", "peer-collection", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceListRouteCandidatesProcedure:             resourceAction("transport.route_candidates", "transport", "network", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceResolveRecordProcedure:                   resourceAction("discovery.resolve_record", "discovery", "discovery-record", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceResolveServiceProcedure:                  resourceAction("discovery.resolve_service", "discovery", "service", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceListRecordsProcedure:                     resourceAction("discovery.list_records", "discovery", "discovery-record-collection", identityapi.AccessRead),
	ardentsv1connect.NetworkServiceImportRecordProcedure:                    resourceAction("discovery.import", "discovery", "discovery-record", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure:               resourceAction("workload.register", "workload", "workload", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceStartWorkloadProcedure:                  resourceAction("workload.start", "workload", "workload", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceStopWorkloadProcedure:                   resourceAction("workload.stop", "workload", "workload", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceRestartWorkloadProcedure:                resourceAction("workload.restart", "workload", "workload", identityapi.AccessWrite),
	ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure:              resourceAction("workload.status", "workload", "workload", identityapi.AccessRead),
	ardentsv1connect.WorkloadServiceListWorkloadsProcedure:                  resourceAction("workload.list", "workload", "workload-collection", identityapi.AccessRead),
	ardentsv1connect.WorkloadServiceGetHostedServiceProcedure:               resourceAction("workload.hosted_service", "workload", "service", identityapi.AccessRead),
	ardentsv1connect.WorkloadServiceListHostedServicesProcedure:             resourceAction("workload.hosted_services", "workload", "service-collection", identityapi.AccessRead),
	ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure:    resourceAction("workload.service_publication", "workload", "service", identityapi.AccessRead),
	ardentsv1connect.ContentServicePublishObjectProcedure:                   resourceAction("data.publish_object", "data", "content-object", identityapi.AccessWrite),
	ardentsv1connect.ContentServiceGetObjectProcedure:                       resourceAction("data.get_object", "data", "content-object", identityapi.AccessRead),
	ardentsv1connect.ContentServiceListObjectsProcedure:                     resourceAction("data.list_objects", "data", "content-object-collection", identityapi.AccessRead),
	ardentsv1connect.ContentServicePublishBlobProcedure:                     resourceAction("data.publish_blob", "data", "content-blob", identityapi.AccessWrite),
	ardentsv1connect.TransferServiceFetchBlobProcedure:                      resourceAction("data.fetch_blob", "data", "content-blob", identityapi.AccessWrite),
	ardentsv1connect.ContentServiceGetBlobProcedure:                         resourceAction("data.get_blob", "data", "content-blob", identityapi.AccessRead),
	ardentsv1connect.ContentServiceListBlobsProcedure:                       resourceAction("data.list_blobs", "data", "content-blob-collection", identityapi.AccessRead),
	ardentsv1connect.TransferServiceListBlobSourcesProcedure:                resourceAction("data.blob_sources", "data", "content-blob", identityapi.AccessRead),
	ardentsv1connect.TransferServiceGetTransferProcedure:                    resourceAction("data.get_transfer", "data", "transfer", identityapi.AccessRead),
	ardentsv1connect.TransferServiceListTransfersProcedure:                  resourceAction("data.list_transfers", "data", "transfer-collection", identityapi.AccessRead),
	ardentsv1connect.ContentServicePublishManifestProcedure:                 resourceAction("data.publish_manifest", "data", "content-manifest", identityapi.AccessWrite),
	ardentsv1connect.ContentServiceGetManifestProcedure:                     resourceAction("data.get_manifest", "data", "content-manifest", identityapi.AccessRead),
	ardentsv1connect.ContentServiceListManifestsProcedure:                   resourceAction("data.list_manifests", "data", "content-manifest-collection", identityapi.AccessRead),
	ardentsv1connect.RetentionServiceRetainBlobProcedure:                    resourceAction("data.retain_blob", "data", "content-blob", identityapi.AccessWrite),
	ardentsv1connect.RetentionServicePinBlobProcedure:                       resourceAction("data.pin_blob", "data", "content-blob", identityapi.AccessWrite),
	ardentsv1connect.RetentionServiceDropBlobProcedure:                      resourceAction("data.drop_blob", "data", "content-blob", identityapi.AccessWrite),
	ardentsv1connect.ContentServiceGetDataInventoryProcedure:                resourceAction("data.inventory", "data", "content-inventory", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceGetDiagnosticsProcedure:              resourceAction("diagnostics.snapshot", "diagnostics", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceGetPendingOperationsProcedure:        resourceAction("diagnostics.pending_operations", "diagnostics", "operation-collection", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceGetHealthSummaryProcedure:            resourceAction("diagnostics.health_summary", "diagnostics", "diagnostics", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceExplainFailureProcedure:              resourceAction("diagnostics.explain_failure", "diagnostics", "diagnostic-subject", identityapi.AccessRead),
	ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure:            resourceAction("diagnostics.recent_events", "diagnostics", "event-collection", identityapi.AccessRead),
}

func action(name, domain string, access identityapi.Access) Rule {
	return Rule{Action: name, Domain: domain, Access: access}
}

func resourceAction(name, domain, kind string, access identityapi.Access) Rule {
	return Rule{Action: name, Domain: domain, ResourceKind: kind, Access: access}
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
