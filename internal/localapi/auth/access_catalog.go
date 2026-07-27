// Package auth owns the frozen Operator action catalogue and canonical request mapping.
// It does not own identity state or product authorization decisions.
package auth

import (
	"errors"
	"fmt"
	"sort"

	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

var ErrUnknownProcedure = errors.New("operator procedure is not registered")

type Rule struct {
	Action       string
	Domain       string
	ResourceKind string
	Mutating     bool
}

var procedureAccess = map[string]Rule{
	ardentsv1connect.NodeServiceStartNodeProcedure:                          mutationAction("node.start", "node", "node"),
	ardentsv1connect.NodeServiceStopNodeProcedure:                           mutationAction("node.stop", "node", "node"),
	ardentsv1connect.NodeServiceGetNodeStatusProcedure:                      resourceAction("node.status", "node", "node"),
	ardentsv1connect.NodeServiceGetNodeFeaturesProcedure:                    resourceAction("node.features", "node", "node"),
	ardentsv1connect.NodeServiceGetNodeRuntimeProcedure:                     resourceAction("node.runtime", "node", "node"),
	ardentsv1connect.ConfigurationServiceGetEffectiveConfigurationProcedure: resourceAction("config.effective", "config", "configuration"),
	ardentsv1connect.ConfigurationServiceReloadConfigurationProcedure:       mutationAction("config.reload", "config", "configuration"),
	ardentsv1connect.NodeServiceStreamNodeEventsProcedure:                   resourceAction("node.events", "node", "node"),
	ardentsv1connect.NetworkServiceGetNetworkStatusProcedure:                resourceAction("transport.network_status", "transport", "network"),
	ardentsv1connect.NetworkServiceGetDiscoveryStatusProcedure:              resourceAction("discovery.status", "discovery", "discovery-status"),
	ardentsv1connect.NetworkServiceGetLocalPresenceProcedure:                resourceAction("discovery.local_presence", "discovery", "local-presence"),
	ardentsv1connect.NetworkServiceListPeersProcedure:                       resourceAction("discovery.peers", "discovery", "peer-collection"),
	ardentsv1connect.NetworkServiceListRouteCandidatesProcedure:             resourceAction("transport.route_candidates", "transport", "network"),
	ardentsv1connect.NetworkServiceResolveRecordProcedure:                   resourceAction("discovery.resolve_record", "discovery", "discovery-record"),
	ardentsv1connect.NetworkServiceResolveServiceProcedure:                  resourceAction("discovery.resolve_service", "discovery", "service"),
	ardentsv1connect.NetworkServiceListRecordsProcedure:                     resourceAction("discovery.list_records", "discovery", "discovery-record-collection"),
	ardentsv1connect.NetworkServiceImportRecordProcedure:                    mutationAction("discovery.import", "discovery", "discovery-record"),
	ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure:               mutationAction("workload.register", "workload", "workload"),
	ardentsv1connect.WorkloadServiceStartWorkloadProcedure:                  mutationAction("workload.start", "workload", "workload"),
	ardentsv1connect.WorkloadServiceStopWorkloadProcedure:                   mutationAction("workload.stop", "workload", "workload"),
	ardentsv1connect.WorkloadServiceRestartWorkloadProcedure:                mutationAction("workload.restart", "workload", "workload"),
	ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure:              resourceAction("workload.status", "workload", "workload"),
	ardentsv1connect.WorkloadServiceListWorkloadsProcedure:                  resourceAction("workload.list", "workload", "workload-collection"),
	ardentsv1connect.WorkloadServiceGetHostedServiceProcedure:               resourceAction("workload.hosted_service", "workload", "service"),
	ardentsv1connect.WorkloadServiceListHostedServicesProcedure:             resourceAction("workload.hosted_services", "workload", "service-collection"),
	ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure:    resourceAction("workload.service_publication", "workload", "service"),
	ardentsv1connect.ContentServicePublishObjectProcedure:                   mutationAction("data.publish_object", "data", "content-object"),
	ardentsv1connect.ContentServiceGetObjectProcedure:                       resourceAction("data.get_object", "data", "content-object"),
	ardentsv1connect.ContentServiceListObjectsProcedure:                     resourceAction("data.list_objects", "data", "content-object-collection"),
	ardentsv1connect.ContentServicePublishBlobProcedure:                     mutationAction("data.publish_blob", "data", "content-blob"),
	ardentsv1connect.TransferServiceFetchBlobProcedure:                      mutationAction("data.fetch_blob", "data", "content-blob"),
	ardentsv1connect.ContentServiceGetBlobProcedure:                         resourceAction("data.get_blob", "data", "content-blob"),
	ardentsv1connect.ContentServiceListBlobsProcedure:                       resourceAction("data.list_blobs", "data", "content-blob-collection"),
	ardentsv1connect.TransferServiceListBlobSourcesProcedure:                resourceAction("data.blob_sources", "data", "content-blob"),
	ardentsv1connect.TransferServiceGetTransferProcedure:                    resourceAction("data.get_transfer", "data", "transfer"),
	ardentsv1connect.TransferServiceListTransfersProcedure:                  resourceAction("data.list_transfers", "data", "transfer-collection"),
	ardentsv1connect.ContentServicePublishManifestProcedure:                 mutationAction("data.publish_manifest", "data", "content-manifest"),
	ardentsv1connect.ContentServiceGetManifestProcedure:                     resourceAction("data.get_manifest", "data", "content-manifest"),
	ardentsv1connect.ContentServiceListManifestsProcedure:                   resourceAction("data.list_manifests", "data", "content-manifest-collection"),
	ardentsv1connect.RetentionServiceRetainBlobProcedure:                    mutationAction("data.retain_blob", "data", "content-blob"),
	ardentsv1connect.RetentionServicePinBlobProcedure:                       mutationAction("data.pin_blob", "data", "content-blob"),
	ardentsv1connect.RetentionServiceDropBlobProcedure:                      mutationAction("data.drop_blob", "data", "content-blob"),
	ardentsv1connect.ContentServiceGetDataInventoryProcedure:                resourceAction("data.inventory", "data", "content-inventory"),
	ardentsv1connect.DiagnosticsServiceGetDiagnosticsProcedure:              resourceAction("diagnostics.snapshot", "diagnostics", "diagnostics"),
	ardentsv1connect.DiagnosticsServiceGetPendingOperationsProcedure:        resourceAction("diagnostics.pending_operations", "diagnostics", "operation-collection"),
	ardentsv1connect.DiagnosticsServiceGetHealthSummaryProcedure:            resourceAction("diagnostics.health_summary", "diagnostics", "diagnostics"),
	ardentsv1connect.DiagnosticsServiceExplainFailureProcedure:              resourceAction("diagnostics.explain_failure", "diagnostics", "diagnostic-subject"),
	ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure:            resourceAction("diagnostics.recent_events", "diagnostics", "event-collection"),
}

func init() {
	if err := registerProtocolAccess("ardents.v1.AuthorityService"); err != nil {
		panic(err)
	}
}

func registerProtocolAccess(serviceName protoreflect.FullName) error {
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(serviceName)
	if err != nil {
		return fmt.Errorf("load Operator access service %s: %w", serviceName, err)
	}
	service, ok := descriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("Operator access descriptor %s is not a service", serviceName)
	}
	for index := 0; index < service.Methods().Len(); index++ {
		method := service.Methods().Get(index)
		options, ok := method.Options().(*descriptorpb.MethodOptions)
		if !ok || !proto.HasExtension(options, protocol.E_OperatorAccess) {
			return fmt.Errorf("Operator procedure %s.%s has no access metadata", serviceName, method.Name())
		}
		access, ok := proto.GetExtension(options, protocol.E_OperatorAccess).(*protocol.OperatorAccess)
		if !ok || access.GetAction() == "" || access.GetDomain() == "" || access.GetResourceKind() == "" {
			return fmt.Errorf("Operator procedure %s.%s has invalid access metadata", serviceName, method.Name())
		}
		procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
		if _, duplicate := procedureAccess[procedure]; duplicate {
			return fmt.Errorf("Operator procedure %s has duplicate access metadata", procedure)
		}
		procedureAccess[procedure] = Rule{
			Action: access.GetAction(), Domain: access.GetDomain(),
			ResourceKind: access.GetResourceKind(), Mutating: access.GetMutating(),
		}
	}
	return nil
}

func resourceAction(name, domain, kind string) Rule {
	return Rule{Action: name, Domain: domain, ResourceKind: kind}
}

func mutationAction(name, domain, kind string) Rule {
	rule := resourceAction(name, domain, kind)
	rule.Mutating = true
	return rule
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
