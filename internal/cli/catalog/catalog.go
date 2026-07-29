// Package catalog owns the closed Operator CLI command metadata contract.
// It does not dispatch commands; execution remains owned by the command packages.
package catalog

import (
	"fmt"
	"sort"
	"strings"

	"ardents/internal/localapi/protocol/ardentsv1connect"
)

type OutputShape string

const (
	OutputProtoJSON OutputShape = "proto-json-v1"
	OutputCLIJSON   OutputShape = "cli-json-v1"
	OutputJSONLines OutputShape = "json-lines-v1"
	OutputHumanOnly OutputShape = "human-only"
)

type AccessClass string

const (
	AccessOffline          AccessClass = "offline"
	AccessLocal            AccessClass = "local"
	AccessPublicBounded    AccessClass = "public-bounded"
	AccessSessionLifecycle AccessClass = "session-lifecycle"
	AccessProtected        AccessClass = "protected"
	AccessInteractive      AccessClass = "interactive"
)

type CommandSpec struct {
	ID                    string
	Path                  []string
	Usage                 string
	Summary               string
	Procedure             string
	SecondaryProcedures   []string
	ProcedureRequirements []ProcedureRequirement
	Action                string
	ResourceKind          string
	Mutating              bool
	Output                OutputShape
	WatchOutput           OutputShape
	SSH                   bool
	EvidenceOwner         string
	Access                AccessClass
}

// ProcedureRequirement declares one exact protected call made by an aggregate
// command whose constituent procedures have different access metadata.
type ProcedureRequirement struct {
	Procedure    string
	Access       AccessClass
	Action       string
	ResourceKind string
	Mutating     bool
}

type GroupSpec struct {
	Name    string
	Summary string
}

type ProcedureRule struct {
	Access       AccessClass
	Action       string
	ResourceKind string
	Mutating     bool
}

type ProcedureResolver func(string) (ProcedureRule, bool)

const (
	ownerContract            = "OCS-01"
	ownerNND                 = "OCS-02"
	ownerWorkload            = "OCS-03"
	ownerData                = "OCS-04"
	ownerIdentity            = "OCS-05"
	ownerAuthority           = "CGA-01"
	ownerAuthorityDelivery   = "CGA-02"
	ownerAuthorityRotation   = "CGA-03"
	ownerAuthorityMembership = "CGA-04"
	ownerAuthorityRenewal    = "CGA-05"
	ownerAuthorityRecovery   = "CGA-06"
)

var groups = []GroupSpec{
	{Name: "node", Summary: "node lifecycle, runtime status and events"},
	{Name: "network", Summary: "network, discovery, peers and routes"},
	{Name: "topology", Summary: "bounded multi-host deployment status"},
	{Name: "workload", Summary: "workload lifecycle and hosted services"},
	{Name: "data", Summary: "objects, blobs, manifests and transfers"},
	{Name: "diagnostics", Summary: "health, failures, pending operations and events"},
	{Name: "config", Summary: "effective Operator configuration and atomic reload"},
	{Name: "authority", Summary: "single-Realm Channel Grant Authority genesis and inspection"},
	{Name: "identity", Summary: "Principal custody, enrollment, sessions and access administration"},
	{Name: "shell", Summary: "interactive terminal session over the current Operator context"},
	{Name: "tui", Summary: "optional fullscreen Operator dashboard"},
	{Name: "version", Summary: "binary version, commit, build date and target platform"},
}

var commands = []CommandSpec{
	protected("node.start", []string{"node", "start"}, "", "start the Node", ardentsv1connect.NodeServiceStartNodeProcedure, "node.start", "node", true, OutputProtoJSON, ownerNND),
	protected("node.stop", []string{"node", "stop"}, "", "stop the Node", ardentsv1connect.NodeServiceStopNodeProcedure, "node.stop", "node", true, OutputProtoJSON, ownerNND),
	protected("node.status", []string{"node", "status"}, "", "show lifecycle and readiness", ardentsv1connect.NodeServiceGetNodeStatusProcedure, "node.status", "node", false, OutputProtoJSON, ownerNND),
	protected("node.runtime", []string{"node", "runtime"}, "", "show boot, identity and runtime health", ardentsv1connect.NodeServiceGetNodeRuntimeProcedure, "node.runtime", "node", false, OutputProtoJSON, ownerNND),
	protected("node.features", []string{"node", "features"}, "", "show versioned service features", ardentsv1connect.NodeServiceGetNodeFeaturesProcedure, "node.features", "node", false, OutputProtoJSON, ownerNND),
	protected("node.events", []string{"node", "events"}, "[--limit N]", "stream Node events (requires global --watch)", ardentsv1connect.NodeServiceStreamNodeEventsProcedure, "node.events", "node", false, OutputJSONLines, ownerNND),

	withWatch(protected("network.status", []string{"network", "status"}, "", "show network participation", ardentsv1connect.NetworkServiceGetNetworkStatusProcedure, "transport.network_status", "network", false, OutputProtoJSON, ownerNND)),
	protected("network.discovery", []string{"network", "discovery"}, "", "show discovery state", ardentsv1connect.NetworkServiceGetDiscoveryStatusProcedure, "discovery.status", "discovery-status", false, OutputProtoJSON, ownerNND),
	protected("network.presence", []string{"network", "presence"}, "", "show local presence publication", ardentsv1connect.NetworkServiceGetLocalPresenceProcedure, "discovery.local_presence", "local-presence", false, OutputProtoJSON, ownerNND),
	protected("network.peers", []string{"network", "peers"}, "", "list network peers", ardentsv1connect.NetworkServiceListPeersProcedure, "discovery.peers", "peer-collection", false, OutputProtoJSON, ownerNND),
	protected("network.routes", []string{"network", "routes"}, "[--service TYPE]", "list route candidates", ardentsv1connect.NetworkServiceListRouteCandidatesProcedure, "transport.route_candidates", "network", false, OutputProtoJSON, ownerNND),
	protected("network.resolve.record", []string{"network", "resolve", "record"}, "--subject ID --kind KIND", "resolve one signed discovery record", ardentsv1connect.NetworkServiceResolveRecordProcedure, "discovery.resolve_record", "discovery-record", false, OutputProtoJSON, ownerNND),
	protected("network.resolve.service", []string{"network", "resolve", "service"}, "--service ID", "resolve one service", ardentsv1connect.NetworkServiceResolveServiceProcedure, "discovery.resolve_service", "service", false, OutputProtoJSON, ownerNND),
	protected("network.records.list", []string{"network", "records", "list"}, "", "list signed discovery records", ardentsv1connect.NetworkServiceListRecordsProcedure, "discovery.list_records", "discovery-record-collection", false, OutputProtoJSON, ownerNND),
	protected("network.records.import", []string{"network", "records", "import"}, "--file FILE", "import a signed discovery record", ardentsv1connect.NetworkServiceImportRecordProcedure, "discovery.import", "discovery-record", true, OutputProtoJSON, ownerNND),

	protectedAggregate(
		"topology.status",
		[]string{"topology", "status"},
		"--manifest FILE",
		"show bounded three-Node deployment truth",
		[]ProcedureRequirement{
			{Procedure: ardentsv1connect.NodeServiceGetNodeRuntimeProcedure, Access: AccessProtected, Action: "node.runtime", ResourceKind: "node"},
			{Procedure: ardentsv1connect.NetworkServiceGetNetworkStatusProcedure, Access: AccessProtected, Action: "transport.network_status", ResourceKind: "network"},
			{Procedure: ardentsv1connect.NodeServiceGetNodeFeaturesProcedure, Access: AccessProtected, Action: "node.features", ResourceKind: "node"},
			{Procedure: ardentsv1connect.IdentityServiceEndSessionProcedure, Access: AccessSessionLifecycle, Mutating: true},
		},
		OutputCLIJSON,
		ownerContract,
	),
	protectedAggregate(
		"topology.recover",
		[]string{"topology", "recover"},
		"--manifest FILE",
		"verify designated Authority recovery against immutable checkpoint truth",
		[]ProcedureRequirement{
			{Procedure: ardentsv1connect.NodeServiceGetNodeRuntimeProcedure, Access: AccessProtected, Action: "node.runtime", ResourceKind: "node"},
			{Procedure: ardentsv1connect.AuthorityServiceInspectRealmAuthorityProcedure, Access: AccessProtected, Action: "realm.channel.audit.read", ResourceKind: "realm"},
			{Procedure: ardentsv1connect.AuthorityServiceVerifyRestoredAuthorityProcedure, Access: AccessProtected, Action: "realm.channel.recovery.execute", ResourceKind: "realm", Mutating: true},
			{Procedure: ardentsv1connect.IdentityServiceEndSessionProcedure, Access: AccessSessionLifecycle, Mutating: true},
		},
		OutputCLIJSON,
		ownerAuthorityRecovery,
	),

	protected("workload.list", []string{"workload", "list"}, "", "list workloads", ardentsv1connect.WorkloadServiceListWorkloadsProcedure, "workload.list", "workload-collection", false, OutputProtoJSON, ownerWorkload),
	protected("workload.get", []string{"workload", "get"}, "ID", "show one workload", ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure, "workload.status", "workload", false, OutputProtoJSON, ownerWorkload),
	protected("workload.register", []string{"workload", "register"}, "--file FILE", "register a workload specification", ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure, "workload.register", "workload", true, OutputProtoJSON, ownerWorkload),
	protected("workload.start", []string{"workload", "start"}, "ID", "start a workload", ardentsv1connect.WorkloadServiceStartWorkloadProcedure, "workload.start", "workload", true, OutputProtoJSON, ownerWorkload),
	protected("workload.stop", []string{"workload", "stop"}, "ID", "stop a workload", ardentsv1connect.WorkloadServiceStopWorkloadProcedure, "workload.stop", "workload", true, OutputProtoJSON, ownerWorkload),
	protected("workload.restart", []string{"workload", "restart"}, "ID", "restart a workload", ardentsv1connect.WorkloadServiceRestartWorkloadProcedure, "workload.restart", "workload", true, OutputProtoJSON, ownerWorkload),
	protected("workload.services", []string{"workload", "services"}, "", "list hosted services", ardentsv1connect.WorkloadServiceListHostedServicesProcedure, "workload.hosted_services", "service-collection", false, OutputProtoJSON, ownerWorkload),
	protected("workload.service", []string{"workload", "service"}, "ID", "show hosted-service readiness", ardentsv1connect.WorkloadServiceGetHostedServiceProcedure, "workload.hosted_service", "service", false, OutputProtoJSON, ownerWorkload),
	protected("workload.publication", []string{"workload", "publication"}, "ID", "show service publication state", ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure, "workload.service_publication", "service", false, OutputProtoJSON, ownerWorkload),

	protected("data.inventory", []string{"data", "inventory"}, "", "show content inventory", ardentsv1connect.ContentServiceGetDataInventoryProcedure, "data.inventory", "content-inventory", false, OutputProtoJSON, ownerData),
	protected("data.objects.list", []string{"data", "objects", "list"}, "", "list content objects", ardentsv1connect.ContentServiceListObjectsProcedure, "data.list_objects", "content-object-collection", false, OutputProtoJSON, ownerData),
	protected("data.objects.get", []string{"data", "objects", "get"}, "ID", "show one content object", ardentsv1connect.ContentServiceGetObjectProcedure, "data.get_object", "content-object", false, OutputProtoJSON, ownerData),
	protected("data.objects.publish", []string{"data", "objects", "publish"}, "--file FILE", "publish a content object", ardentsv1connect.ContentServicePublishObjectProcedure, "data.publish_object", "content-object", true, OutputProtoJSON, ownerData),
	protected("data.blobs.list", []string{"data", "blobs", "list"}, "", "list Blobs", ardentsv1connect.ContentServiceListBlobsProcedure, "data.list_blobs", "content-blob-collection", false, OutputProtoJSON, ownerData),
	protected("data.blobs.get", []string{"data", "blobs", "get"}, "ID", "show one Blob", ardentsv1connect.ContentServiceGetBlobProcedure, "data.get_blob", "content-blob", false, OutputProtoJSON, ownerData),
	protected("data.blobs.publish", []string{"data", "blobs", "publish"}, "--file FILE", "publish Blob payload", ardentsv1connect.ContentServicePublishBlobProcedure, "data.publish_blob", "content-blob", true, OutputProtoJSON, ownerData),
	protected("data.blobs.fetch", []string{"data", "blobs", "fetch"}, "ID", "start a private Blob fetch", ardentsv1connect.TransferServiceFetchBlobProcedure, "data.fetch_blob", "content-blob", true, OutputProtoJSON, ownerData),
	protected("data.blobs.sources", []string{"data", "blobs", "sources"}, "ID", "list eligible Blob sources", ardentsv1connect.TransferServiceListBlobSourcesProcedure, "data.blob_sources", "content-blob", false, OutputProtoJSON, ownerData),
	protected("data.blobs.retain", []string{"data", "blobs", "retain"}, "--id ID --expires-at TIME", "set Blob retention", ardentsv1connect.RetentionServiceRetainBlobProcedure, "data.retain_blob", "content-blob", true, OutputProtoJSON, ownerData),
	protected("data.blobs.pin", []string{"data", "blobs", "pin"}, "ID", "pin a Blob", ardentsv1connect.RetentionServicePinBlobProcedure, "data.pin_blob", "content-blob", true, OutputProtoJSON, ownerData),
	protected("data.blobs.drop", []string{"data", "blobs", "drop"}, "ID", "drop local Blob retention", ardentsv1connect.RetentionServiceDropBlobProcedure, "data.drop_blob", "content-blob", true, OutputProtoJSON, ownerData),
	protected("data.manifests.list", []string{"data", "manifests", "list"}, "", "list content Manifests", ardentsv1connect.ContentServiceListManifestsProcedure, "data.list_manifests", "content-manifest-collection", false, OutputProtoJSON, ownerData),
	protected("data.manifests.get", []string{"data", "manifests", "get"}, "ID", "show one content Manifest", ardentsv1connect.ContentServiceGetManifestProcedure, "data.get_manifest", "content-manifest", false, OutputProtoJSON, ownerData),
	protected("data.manifests.publish", []string{"data", "manifests", "publish"}, "--file FILE", "publish a content Manifest", ardentsv1connect.ContentServicePublishManifestProcedure, "data.publish_manifest", "content-manifest", true, OutputProtoJSON, ownerData),
	withWatch(protected("data.transfers.list", []string{"data", "transfers", "list"}, "", "list transfers", ardentsv1connect.TransferServiceListTransfersProcedure, "data.list_transfers", "transfer-collection", false, OutputProtoJSON, ownerData)),
	withWatch(protected("data.transfers.get", []string{"data", "transfers", "get"}, "ID", "show transfer progress", ardentsv1connect.TransferServiceGetTransferProcedure, "data.get_transfer", "transfer", false, OutputProtoJSON, ownerData)),

	protected("diagnostics.snapshot", []string{"diagnostics", "snapshot"}, "", "show combined diagnostics", ardentsv1connect.DiagnosticsServiceGetDiagnosticsProcedure, "diagnostics.snapshot", "diagnostics", false, OutputProtoJSON, ownerNND),
	withWatch(protected("diagnostics.health", []string{"diagnostics", "health"}, "", "show readiness causes", ardentsv1connect.DiagnosticsServiceGetHealthSummaryProcedure, "diagnostics.health_summary", "diagnostics", false, OutputProtoJSON, ownerNND)),
	protected("diagnostics.pending", []string{"diagnostics", "pending"}, "", "list pending operations", ardentsv1connect.DiagnosticsServiceGetPendingOperationsProcedure, "diagnostics.pending_operations", "operation-collection", false, OutputProtoJSON, ownerNND),
	protected("diagnostics.explain", []string{"diagnostics", "explain"}, "--scope S [--resource-id ID]", "explain one diagnostic subject", ardentsv1connect.DiagnosticsServiceExplainFailureProcedure, "diagnostics.explain_failure", "diagnostic-subject", false, OutputProtoJSON, ownerNND),
	protected("diagnostics.events", []string{"diagnostics", "events"}, "[--limit N] [--cursor C]", "list recent events", ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure, "diagnostics.recent_events", "event-collection", false, OutputProtoJSON, ownerNND),

	protected("config.show", []string{"config", "show"}, "", "show effective configuration", ardentsv1connect.ConfigurationServiceGetEffectiveConfigurationProcedure, "config.effective", "configuration", false, OutputProtoJSON, ownerContract),
	protected("config.reload", []string{"config", "reload"}, "", "reload configuration atomically", ardentsv1connect.ConfigurationServiceReloadConfigurationProcedure, "config.reload", "configuration", true, OutputProtoJSON, ownerContract),

	protected("authority.create", []string{"authority", "create"}, "--request-id ID", "create or reopen the single Realm Authority", ardentsv1connect.AuthorityServiceCreateRealmAuthorityProcedure, "realm.authority.create", "realm-authority-instance", true, OutputProtoJSON, ownerAuthority),
	protected("authority.inspect", []string{"authority", "inspect"}, "--realm-id ID", "show bounded Realm Authority readiness", ardentsv1connect.AuthorityServiceInspectRealmAuthorityProcedure, "realm.channel.audit.read", "realm", false, OutputProtoJSON, ownerAuthority),
	protected("authority.recovery.verify", []string{"authority", "recovery", "verify"}, "--realm-id ID --authority-sequence N --checkpoint-digest DIGEST", "verify an exact recovery-only restore against the independent head", ardentsv1connect.AuthorityServiceVerifyRestoredAuthorityProcedure, "realm.channel.recovery.execute", "realm", true, OutputProtoJSON, ownerAuthorityRecovery),
	protected("authority.channel.inspect", []string{"authority", "channel", "inspect"}, "--realm-id ID --channel-id HEX", "show one redacted channel lifecycle status", ardentsv1connect.AuthorityServiceInspectChannelProcedure, "realm.channel.audit.read", "realm-channel", false, OutputProtoJSON, ownerAuthorityRenewal),
	protected("authority.delivery.prepare", []string{"authority", "delivery", "prepare"}, "--subject ID --valid-for DURATION --out-file FILE", "prepare a recipient delivery-key attestation", ardentsv1connect.ChannelDeliveryServicePrepareGenerationDeliveryProcedure, "realm.channel.delivery.prepare", "principal", true, OutputProtoJSON, ownerAuthorityDelivery),
	protected("authority.delivery.issue", []string{"authority", "delivery", "issue"}, "--realm-id ID --request-id ID --channel-class CLASS --permissions N --valid-for DURATION --attestation-file FILE --out-file FILE", "issue one recipient-bound initial generation", ardentsv1connect.AuthorityServiceIssueInitialGenerationProcedure, "realm.channel.delivery.issue", "realm-channel-delivery", true, OutputProtoJSON, ownerAuthorityDelivery),
	protected("authority.delivery.install", []string{"authority", "delivery", "install"}, "--delivery-file FILE --out-file FILE", "atomically install a sealed initial generation", ardentsv1connect.ChannelDeliveryServiceInstallGenerationDeliveryProcedure, "realm.channel.delivery.install", "realm-channel-delivery", true, OutputProtoJSON, ownerAuthorityDelivery),
	protected("authority.delivery.acknowledge", []string{"authority", "delivery", "acknowledge"}, "--delivery-file FILE --receipt-file FILE", "acknowledge one installed initial generation", ardentsv1connect.AuthorityServiceAcknowledgeInitialGenerationProcedure, "realm.channel.delivery.acknowledge", "realm-channel-delivery", true, OutputProtoJSON, ownerAuthorityDelivery),
	protected("authority.rotation.rotate", []string{"authority", "rotation", "rotate"}, "--realm-id ID --request-id ID --channel-id HEX --attestation-file FILE --valid-for DURATION --drain-for DURATION --out-file FILE", "create and seal one fresh pending generation", ardentsv1connect.AuthorityServiceRotateChannelProcedure, "realm.channel.generation.rotate", "realm-channel", true, OutputProtoJSON, ownerAuthorityRotation),
	protected("authority.rotation.renew", []string{"authority", "rotation", "renew"}, "--realm-id ID --request-id ID --channel-id HEX --attestation-file FILE --drain-for DURATION --out-file FILE", "renew grants inside the bounded threshold", ardentsv1connect.AuthorityServiceRenewChannelGrantsProcedure, "realm.channel.generation.rotate", "realm-channel", true, OutputProtoJSON, ownerAuthorityRenewal),
	protected("authority.rotation.install", []string{"authority", "rotation", "install"}, "--rotation-file FILE [--recipient ID] --out-file FILE", "install a sealed pending generation", ardentsv1connect.ChannelDeliveryServiceInstallGenerationDeliveryProcedure, "realm.channel.delivery.install", "realm-channel-delivery", true, OutputProtoJSON, ownerAuthorityRotation),
	protected("authority.rotation.acknowledge-installed", []string{"authority", "rotation", "acknowledge-installed"}, "--rotation-file FILE --receipt-file FILE", "acknowledge pending-generation installation", ardentsv1connect.AuthorityServiceAcknowledgeInitialGenerationProcedure, "realm.channel.delivery.acknowledge", "realm-channel-delivery", true, OutputProtoJSON, ownerAuthorityRotation),
	protected("authority.rotation.commit", []string{"authority", "rotation", "commit"}, "--rotation-file FILE --out-file FILE", "commit the signed generation activation", ardentsv1connect.AuthorityServiceCommitChannelActivationProcedure, "realm.channel.activation.commit", "realm-channel-operation", true, OutputProtoJSON, ownerAuthorityRotation),
	protected("authority.rotation.activate", []string{"authority", "rotation", "activate"}, "--activation-file FILE --out-file FILE", "activate a committed generation on a member", ardentsv1connect.ChannelDeliveryServiceActivateGenerationProcedure, "realm.channel.generation.activate", "realm-channel-operation", true, OutputProtoJSON, ownerAuthorityRotation),
	protected("authority.rotation.acknowledge-active", []string{"authority", "rotation", "acknowledge-active"}, "--rotation-file FILE --receipt-file FILE --host-disposition approved", "record deployment-approved host activation", ardentsv1connect.AuthorityServiceAcknowledgeChannelActivationProcedure, "realm.channel.activation.acknowledge", "realm-channel-delivery", true, OutputProtoJSON, ownerAuthorityRotation),
	protected("authority.membership.change", []string{"authority", "membership", "change"}, "--realm-id ID --request-id ID --channel-id HEX --change add|remove --target-principal ID --attestation-file FILE --valid-for DURATION --drain-for DURATION --out-file FILE", "change channel membership with a fresh generation", ardentsv1connect.AuthorityServiceChangeChannelMembershipProcedure, "realm.channel.membership.change", "realm-channel", true, OutputProtoJSON, ownerAuthorityMembership),
	protected("authority.membership.fence", []string{"authority", "membership", "fence"}, "--realm-id ID --channel-id HEX --operation-id ID --evidence-file FILE", "submit bounded deployment fencing evidence", ardentsv1connect.AuthorityServiceSubmitDeploymentFenceEvidenceProcedure, "realm.channel.membership.change", "realm-channel", true, OutputProtoJSON, ownerAuthorityMembership),

	offline("identity.principal.create", []string{"identity", "principal", "create"}, "[--signer-file PATH]", "create an offline Principal root", "offline.identity.principal.create", OutputCLIJSON),
	offline("identity.principal.import", []string{"identity", "principal", "import"}, "--from-file PATH [--signer-file PATH]", "import a protected Principal root", "offline.identity.principal.import", OutputCLIJSON),
	offline("identity.principal.show", []string{"identity", "principal", "show"}, "[--signer-file PATH]", "show public Principal root metadata", "offline.identity.principal.show", OutputCLIJSON),
	offline("identity.device.create", []string{"identity", "device", "create"}, "[--root-signer-file PATH] [--signer-file PATH] [--valid-for DURATION]", "create a finite device Credential", "offline.identity.device.create", OutputCLIJSON),
	offline("identity.device.show", []string{"identity", "device", "show"}, "[--signer-file PATH]", "show public device metadata", "offline.identity.device.show", OutputCLIJSON),
	protected("identity.device.revoke", []string{"identity", "device", "revoke"}, "--principal ID --device-id ID [--request-id ID] [--yes]", "revoke one Principal device", ardentsv1connect.IdentityServiceRevokeDeviceProcedure, "identity.device.revoke", "device", true, OutputCLIJSON, ownerIdentity),
	withSecondary(protected("identity.enroll", []string{"identity", "enroll"}, "[--root-signer-file PATH] [--device-signer-file PATH] [--bootstrap-ticket-file PATH | --request-id ID]", "enroll a Principal on the Node", ardentsv1connect.IdentityServiceEnrollPrincipalProcedure, "identity.principal.enroll", "principal", true, OutputCLIJSON, ownerIdentity), ardentsv1connect.IdentityServiceEnrollFirstPrincipalProcedure),
	protected("identity.grant.list", []string{"identity", "grant", "list"}, "--subject ID", "list Access Grants", ardentsv1connect.IdentityServiceListAccessGrantsProcedure, "identity.grant.list", "grant-collection", false, OutputCLIJSON, ownerIdentity),
	protected("identity.grant.issue", []string{"identity", "grant", "issue"}, "--subject ID --action ACTION [--scope node|exact] [--resource-kind KIND --resource-id ID] [--valid-for DURATION] [--request-id ID] [--yes]", "issue an Access Grant", ardentsv1connect.IdentityServiceIssueAccessGrantProcedure, "identity.grant.issue", "grant-proposal", true, OutputCLIJSON, ownerIdentity),
	protected("identity.grant.revoke", []string{"identity", "grant", "revoke"}, "--subject ID --grant-id ID [--request-id ID] [--yes]", "revoke an Access Grant", ardentsv1connect.IdentityServiceRevokeAccessGrantProcedure, "identity.grant.revoke", "access-grant", true, OutputCLIJSON, ownerIdentity),
	offline("identity.delegation.issue", []string{"identity", "delegation", "issue"}, "--application ID --action ACTION --scope principal-owned|exact --out-file PATH [--resource-kind KIND --resource-id ID] [--valid-for DURATION] [--signer-file PATH] [--yes]", "sign a one-hop Delegation", "offline.identity.delegation.issue", OutputCLIJSON),
	offline("identity.delegation.revoke", []string{"identity", "delegation", "revoke"}, "--delegation-file PATH --out-file PATH [--signer-file PATH] [--yes]", "sign a Delegation revocation", "offline.identity.delegation.revoke", OutputCLIJSON),
	public("identity.delegation.import-revocation", []string{"identity", "delegation", "import-revocation"}, "--revocation-file PATH", "import a signed Delegation revocation", ardentsv1connect.IdentityServiceImportDelegationRevocationProcedure, true, OutputCLIJSON, ownerIdentity),
	protected("identity.application-ticket.issue", []string{"identity", "application-ticket", "issue"}, "--principal ID --action ACTION --out-file PATH [--yes]", "issue an Application Enrollment Ticket", ardentsv1connect.IdentityServiceIssueApplicationEnrollmentTicketProcedure, "identity.principal.enroll", "principal", true, OutputCLIJSON, ownerIdentity),
	withSecondary(public("identity.login", []string{"identity", "login"}, "", "authenticate this process", ardentsv1connect.IdentityServiceCompleteAuthenticationProcedure, true, OutputCLIJSON, ownerIdentity), ardentsv1connect.IdentityServiceBeginAuthenticationProcedure),
	local("identity.status", []string{"identity", "status"}, "", "show process-local Session state", "local.identity.session.status", OutputCLIJSON, true, ownerIdentity),
	session("identity.logout", []string{"identity", "logout"}, "", "end cached Sessions", ardentsv1connect.IdentityServiceEndSessionProcedure, true, OutputCLIJSON, ownerIdentity),

	interactive("shell", []string{"shell"}, "", "open an interactive Operator shell", "interactive.shell", true),
	interactive("tui", []string{"tui"}, "", "open the fullscreen Operator dashboard", "interactive.tui", true),
	local("version", []string{"version"}, "", "show binary build identity", "local.version", OutputCLIJSON, false, ownerContract),
}

var closedError = validateClosed()

func protected(id string, path []string, usage, summary, procedure, action, resource string, mutating bool, shape OutputShape, owner string) CommandSpec {
	return CommandSpec{ID: id, Path: path, Usage: usageText(path, usage), Summary: summary, Procedure: procedure, Action: action, ResourceKind: resource, Mutating: mutating, Output: shape, SSH: true, EvidenceOwner: owner, Access: AccessProtected}
}

func protectedAggregate(
	id string,
	path []string,
	usage, summary string,
	requirements []ProcedureRequirement,
	shape OutputShape,
	owner string,
) CommandSpec {
	return CommandSpec{
		ID: id, Path: path, Usage: usageText(path, usage), Summary: summary,
		ProcedureRequirements: append([]ProcedureRequirement(nil), requirements...),
		Output:                shape, SSH: true, EvidenceOwner: owner, Access: AccessProtected,
	}
}

func offline(id string, path []string, usage, summary, procedure string, shape OutputShape) CommandSpec {
	return CommandSpec{ID: id, Path: path, Usage: usageText(path, usage), Summary: summary, Procedure: procedure, Output: shape, EvidenceOwner: ownerContract, Access: AccessOffline}
}

func public(id string, path []string, usage, summary, procedure string, mutating bool, shape OutputShape, owner string) CommandSpec {
	return CommandSpec{ID: id, Path: path, Usage: usageText(path, usage), Summary: summary, Procedure: procedure, Mutating: mutating, Output: shape, SSH: true, EvidenceOwner: owner, Access: AccessPublicBounded}
}

func session(id string, path []string, usage, summary, procedure string, mutating bool, shape OutputShape, owner string) CommandSpec {
	return CommandSpec{ID: id, Path: path, Usage: usageText(path, usage), Summary: summary, Procedure: procedure, Mutating: mutating, Output: shape, SSH: true, EvidenceOwner: owner, Access: AccessSessionLifecycle}
}

func local(id string, path []string, usage, summary, procedure string, shape OutputShape, ssh bool, owner string) CommandSpec {
	return CommandSpec{ID: id, Path: path, Usage: usageText(path, usage), Summary: summary, Procedure: procedure, Output: shape, SSH: ssh, EvidenceOwner: owner, Access: AccessLocal}
}

func interactive(id string, path []string, usage, summary, procedure string, ssh bool) CommandSpec {
	return CommandSpec{ID: id, Path: path, Usage: usageText(path, usage), Summary: summary, Procedure: procedure, Output: OutputHumanOnly, SSH: ssh, EvidenceOwner: ownerContract, Access: AccessInteractive}
}

func withSecondary(spec CommandSpec, procedures ...string) CommandSpec {
	spec.SecondaryProcedures = append([]string(nil), procedures...)
	return spec
}

func withWatch(spec CommandSpec) CommandSpec {
	spec.WatchOutput = OutputJSONLines
	return spec
}

func usageText(path []string, suffix string) string {
	return strings.TrimSpace(strings.Join(path, " ") + " " + suffix)
}

func Commands() []CommandSpec {
	result := make([]CommandSpec, len(commands))
	for index, spec := range commands {
		result[index] = cloneSpec(spec)
	}
	return result
}

func ClosedError() error { return closedError }

func Groups() []GroupSpec { return append([]GroupSpec(nil), groups...) }

func KnownOutputShapes() []OutputShape {
	return []OutputShape{OutputProtoJSON, OutputCLIJSON, OutputJSONLines, OutputHumanOnly}
}

func PathString(path []string) string { return strings.Join(path, " ") }

func Validate(specs []CommandSpec, resolve ProcedureResolver) error {
	if len(specs) == 0 {
		return fmt.Errorf("command catalogue is empty")
	}
	ids := make(map[string]struct{}, len(specs))
	paths := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		if spec.ID == "" || len(spec.Path) == 0 {
			return fmt.Errorf("command ID and path are required")
		}
		if _, exists := ids[spec.ID]; exists {
			return fmt.Errorf("duplicate command ID %q", spec.ID)
		}
		ids[spec.ID] = struct{}{}
		path := PathString(spec.Path)
		if _, exists := paths[path]; exists {
			return fmt.Errorf("duplicate command path %q", path)
		}
		paths[path] = struct{}{}
		if spec.Usage == "" || spec.Summary == "" {
			return fmt.Errorf("command %q has incomplete help entry", spec.ID)
		}
		if spec.EvidenceOwner == "" {
			return fmt.Errorf("command %q has no evidence owner", spec.ID)
		}
		if !knownOutput(spec.Output) {
			return fmt.Errorf("command %q has unknown output shape %q", spec.ID, spec.Output)
		}
		if spec.WatchOutput != "" && !knownOutput(spec.WatchOutput) {
			return fmt.Errorf("command %q has unknown watch output shape %q", spec.ID, spec.WatchOutput)
		}
		if spec.Output == OutputHumanOnly && spec.Access != AccessInteractive {
			return fmt.Errorf("human-only command %q is not interactive", spec.ID)
		}
		switch spec.Access {
		case AccessProtected:
			if len(spec.ProcedureRequirements) == 0 {
				if spec.Procedure == "" || spec.Action == "" || spec.ResourceKind == "" {
					return fmt.Errorf("protected command %q has incomplete access metadata", spec.ID)
				}
			} else if spec.Procedure != "" || len(spec.SecondaryProcedures) != 0 ||
				spec.Action != "" || spec.ResourceKind != "" || spec.Mutating {
				return fmt.Errorf("protected aggregate %q mixes aggregate and singular access metadata", spec.ID)
			} else if err := validateProcedureRequirements(spec); err != nil {
				return err
			}
			if !spec.SSH {
				return fmt.Errorf("protected command %q has no SSH stream-local support", spec.ID)
			}
		case AccessOffline:
			if !strings.HasPrefix(spec.Procedure, "offline.") || spec.Action != "" || spec.ResourceKind != "" || spec.SSH {
				return fmt.Errorf("offline command %q claims online access", spec.ID)
			}
		case AccessLocal:
			if !strings.HasPrefix(spec.Procedure, "local.") || spec.Action != "" || spec.ResourceKind != "" {
				return fmt.Errorf("local command %q has invalid procedure metadata", spec.ID)
			}
		case AccessInteractive:
			if !strings.HasPrefix(spec.Procedure, "interactive.") || spec.Output != OutputHumanOnly || !spec.SSH {
				return fmt.Errorf("interactive command %q has invalid output metadata", spec.ID)
			}
		case AccessPublicBounded, AccessSessionLifecycle:
			if spec.Procedure == "" || spec.Action != "" || spec.ResourceKind != "" || !spec.SSH {
				return fmt.Errorf("command %q has invalid non-protected access metadata", spec.ID)
			}
		default:
			return fmt.Errorf("command %q has unknown access class %q", spec.ID, spec.Access)
		}
		if resolve != nil {
			if len(spec.ProcedureRequirements) > 0 {
				for _, requirement := range spec.ProcedureRequirements {
					rule, ok := resolve(requirement.Procedure)
					if !ok {
						return fmt.Errorf("command %q procedure %q is not on the Operator surface", spec.ID, requirement.Procedure)
					}
					if rule.Access != requirement.Access ||
						rule.Action != requirement.Action ||
						rule.ResourceKind != requirement.ResourceKind ||
						rule.Mutating != requirement.Mutating {
						return fmt.Errorf("command %q metadata does not match procedure %q", spec.ID, requirement.Procedure)
					}
				}
				continue
			}
			procedures := append([]string{spec.Procedure}, spec.SecondaryProcedures...)
			for _, procedure := range procedures {
				if strings.HasPrefix(procedure, "offline.") || strings.HasPrefix(procedure, "local.") || strings.HasPrefix(procedure, "interactive.") {
					continue
				}
				rule, ok := resolve(procedure)
				if !ok {
					return fmt.Errorf("command %q procedure %q is not on the Operator surface", spec.ID, procedure)
				}
				if procedure == spec.Procedure && (rule.Access != spec.Access || rule.Action != spec.Action || rule.ResourceKind != spec.ResourceKind || rule.Mutating != spec.Mutating) {
					return fmt.Errorf("command %q metadata does not match procedure %q", spec.ID, procedure)
				}
			}
		}
	}
	return nil
}

func validateProcedureRequirements(spec CommandSpec) error {
	seen := make(map[string]struct{}, len(spec.ProcedureRequirements))
	for _, requirement := range spec.ProcedureRequirements {
		if requirement.Procedure == "" {
			return fmt.Errorf("protected aggregate %q has incomplete procedure metadata", spec.ID)
		}
		switch requirement.Access {
		case AccessProtected:
			if requirement.Action == "" || requirement.ResourceKind == "" {
				return fmt.Errorf("protected aggregate %q has incomplete protected procedure metadata", spec.ID)
			}
		case AccessPublicBounded, AccessSessionLifecycle:
			if requirement.Action != "" || requirement.ResourceKind != "" {
				return fmt.Errorf("protected aggregate %q has invalid lifecycle procedure metadata", spec.ID)
			}
		default:
			return fmt.Errorf("protected aggregate %q has invalid procedure access %q", spec.ID, requirement.Access)
		}
		if _, duplicate := seen[requirement.Procedure]; duplicate {
			return fmt.Errorf("protected aggregate %q repeats procedure %q", spec.ID, requirement.Procedure)
		}
		seen[requirement.Procedure] = struct{}{}
	}
	return nil
}

// Procedures returns every procedure a command declares it may call.
func Procedures(spec CommandSpec) []string {
	if len(spec.ProcedureRequirements) > 0 {
		result := make([]string, 0, len(spec.ProcedureRequirements))
		for _, requirement := range spec.ProcedureRequirements {
			result = append(result, requirement.Procedure)
		}
		return result
	}
	return append([]string{spec.Procedure}, spec.SecondaryProcedures...)
}

func ValidateReachability(specs []CommandSpec, reachablePaths []string) error {
	registered := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		registered[PathString(spec.Path)] = struct{}{}
	}
	reachable := make(map[string]struct{}, len(reachablePaths))
	for _, path := range reachablePaths {
		if _, duplicate := reachable[path]; duplicate {
			return fmt.Errorf("parser reports duplicate command path %q", path)
		}
		reachable[path] = struct{}{}
		if _, known := registered[path]; !known {
			return fmt.Errorf("parser command %q is missing from catalogue", path)
		}
	}
	for path := range registered {
		if _, ok := reachable[path]; !ok {
			return fmt.Errorf("catalogue command %q is not reachable", path)
		}
	}
	return nil
}

func knownOutput(shape OutputShape) bool {
	for _, known := range KnownOutputShapes() {
		if shape == known {
			return true
		}
	}
	return false
}

func Match(args []string) (CommandSpec, bool) {
	var matches []CommandSpec
	for _, spec := range commands {
		if len(args) < len(spec.Path) {
			continue
		}
		match := true
		for index := range spec.Path {
			if args[index] != spec.Path[index] {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, spec)
		}
	}
	if len(matches) == 0 {
		return CommandSpec{}, false
	}
	sort.Slice(matches, func(i, j int) bool { return len(matches[i].Path) > len(matches[j].Path) })
	return cloneSpec(matches[0]), true
}

func Exact(path []string) (CommandSpec, bool) {
	for _, spec := range commands {
		if PathString(spec.Path) == PathString(path) {
			return cloneSpec(spec), true
		}
	}
	return CommandSpec{}, false
}

func Under(prefix []string) []CommandSpec {
	result := make([]CommandSpec, 0)
	for _, spec := range Commands() {
		if hasPrefix(spec.Path, prefix) {
			result = append(result, spec)
		}
	}
	sort.Slice(result, func(i, j int) bool { return PathString(result[i].Path) < PathString(result[j].Path) })
	return result
}

func HasDescendant(prefix []string) bool {
	for _, spec := range commands {
		if len(spec.Path) > len(prefix) && hasPrefix(spec.Path, prefix) {
			return true
		}
	}
	return false
}

func TopLevel() []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, spec := range commands {
		name := spec.Path[0]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func validateClosed() error {
	if err := Validate(commands, nil); err != nil {
		return err
	}
	topLevels := TopLevel()
	if len(groups) != len(topLevels) {
		return fmt.Errorf("command group help count is %d, want %d", len(groups), len(topLevels))
	}
	for index, group := range groups {
		if group.Name != topLevels[index] || group.Summary == "" {
			return fmt.Errorf("command group help entry %d does not match %q", index, topLevels[index])
		}
	}
	return nil
}

func cloneSpec(spec CommandSpec) CommandSpec {
	spec.Path = append([]string(nil), spec.Path...)
	spec.SecondaryProcedures = append([]string(nil), spec.SecondaryProcedures...)
	spec.ProcedureRequirements = append([]ProcedureRequirement(nil), spec.ProcedureRequirements...)
	return spec
}

func hasPrefix(path, prefix []string) bool {
	if len(path) < len(prefix) {
		return false
	}
	for index := range prefix {
		if path[index] != prefix[index] {
			return false
		}
	}
	return true
}
