// Package identitycontract is the single named owner of immutable version 1
// identity protocol limits, domains, action names, and resource-kind contracts.
// It does not own identity state, admission, or credential custody.
package identitycontract

import (
	"encoding/base64"
	"strings"
	"time"
)

const (
	Version                                uint32 = 1
	ProtocolMajor                          uint32 = 1
	PortableClockSkew                             = 120 * time.Second
	MaxCredentialLifetime                         = 365 * 24 * time.Hour
	MaxGrantLifetime                              = 365 * 24 * time.Hour
	MaxDelegationLifetime                         = 24 * time.Hour
	MaxKeyCredentialBytes                         = 4 << 10
	MaxArtifactBytes                              = 16 << 10
	MaxActions                                    = 64
	MaxActionBytes                                = 128
	MaxResourceKindBytes                          = 32
	MaxCanonicalResourceIDBytes                   = 512
	MaxOperatorPublishBlobPayloadBytes            = 1 << 20
	MinApplicationDiscoveryAcceptedSchemes        = 1
	MaxApplicationDiscoveryAcceptedSchemes        = 3
	MinApplicationDiscoveryTargets                = 1
	MaxApplicationDiscoveryTargets                = 8
	LowerTimestampUnix                     int64  = 1577836800 // 2020-01-01T00:00:00Z
	UpperTimestampUnix                     int64  = 4102444800 // 2100-01-01T00:00:00Z, exclusive

	KeyCredentialDomain           = "ardents:key-credential:v1\x00"
	AccessGrantDomain             = "ardents:access-grant:v1\x00"
	DelegationDomain              = "ardents:delegation:v1\x00"
	DeviceRevocationDomain        = "ardents:device-revocation:v1\x00"
	AccessGrantRevocationDomain   = "ardents:access-grant-revocation:v1\x00"
	DelegationRevocationDomain    = "ardents:delegation-revocation:v1\x00"
	AuthenticationChallengeDomain = "ardents:authentication-challenge:v1\x00"
	EnrollmentChallengeDomain     = "ardents:enrollment-challenge:v1\x00"
	KeyCredentialPrefix           = "kc1_"
	AccessGrantPrefix             = "ag1_"
	DelegationPrefix              = "dl1_"
	DeviceRevocationPrefix        = "dv1_"
	AccessGrantRevocationPrefix   = "ar1_"
	DelegationRevocationPrefix    = "dr1_"

	ApplicationDiscoverySchemeHTTPS = "https"
	ApplicationDiscoverySchemeHTTP  = "http"
	ApplicationDiscoverySchemeTCP   = "tcp"
)

// EncodeApplicationEnrollmentTicket returns the single canonical text form
// shared by the Operator CLI and the public SDK. The raw ticket must be exactly
// sized and non-zero.
func EncodeApplicationEnrollmentTicket(raw []byte) (string, bool) {
	if len(raw) != ApplicationEnrollmentTicketBytes {
		return "", false
	}
	nonzero := false
	for _, value := range raw {
		nonzero = nonzero || value != 0
	}
	if !nonzero {
		return "", false
	}
	return base64.RawURLEncoding.EncodeToString(raw), true
}

// DecodeApplicationEnrollmentTicket accepts only the canonical unpadded form
// and never trims surrounding bytes.
func DecodeApplicationEnrollmentTicket(encoded string) ([ApplicationEnrollmentTicketBytes]byte, bool) {
	var result [ApplicationEnrollmentTicketBytes]byte
	if encoded == "" || strings.TrimSpace(encoded) != encoded {
		return result, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) != len(result) || base64.RawURLEncoding.EncodeToString(raw) != encoded {
		clear(raw)
		return result, false
	}
	copy(result[:], raw)
	clear(raw)
	if result == [ApplicationEnrollmentTicketBytes]byte{} {
		return [ApplicationEnrollmentTicketBytes]byte{}, false
	}
	return result, true
}

const (
	ChallengeIDBytes                    = 16
	ChallengeNonceBytes                 = 32
	PeerBindingBytes                    = 32
	SessionSecretBytes                  = 32
	ChallengeLifetime                   = 120 * time.Second
	MaxActiveChallenges                 = 4096
	MaxActiveChallengesPerSource        = 8
	BeginRatePerMinute                  = 10
	BeginRateBurst                      = 8
	DefaultSessionLifetime              = 15 * time.Minute
	MaxSessionLifetime                  = 60 * time.Minute
	MaxActiveSessions                   = 16384
	MaxActiveSessionsPerSourceKey       = 16
	BootstrapTicketBytes                = 32
	BootstrapTicketLifetime             = 10 * time.Minute
	ApplicationEnrollmentTicketBytes    = 32
	ApplicationEnrollmentTicketLifetime = 10 * time.Minute
	DefaultGrantLifetime                = 30 * 24 * time.Hour
)

type Interface uint8

const (
	InterfaceOperator    Interface = 1
	InterfaceApplication Interface = 2
)

func IsRegisteredAction(surface Interface, action string) bool {
	if !ValidActionSyntax(action) {
		return false
	}
	if surface == InterfaceApplication {
		_, ok := LookupApplicationAction(action)
		return ok
	}
	if surface == InterfaceOperator {
		_, ok := operatorActions[action]
		return ok
	}
	return false
}

// ApplicationActionContract is the immutable admission classification for one
// registered Application action. Product procedure rules still own their
// classification, while composition validates it against this independent
// action contract.
type ApplicationActionContract struct {
	Mutating bool
}

func LookupApplicationAction(action string) (ApplicationActionContract, bool) {
	if !ValidActionSyntax(action) {
		return ApplicationActionContract{}, false
	}
	contract, ok := applicationActions[action]
	return contract, ok
}

func ValidActionCount(count int) bool { return count > 0 && count <= MaxActions }
func ValidActionSyntax(action string) bool {
	if len(action) == 0 || len(action) > MaxActionBytes {
		return false
	}
	for _, r := range action {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}
func ValidResourceKindSyntax(kind string) bool {
	return len(kind) > 0 && len(kind) <= MaxResourceKindBytes
}
func ValidCanonicalResourceIDSize(size int) bool {
	return size > 0 && size <= MaxCanonicalResourceIDBytes
}
func ValidKeyCredentialSize(size int) bool { return size > 0 && size <= MaxKeyCredentialBytes }
func ValidArtifactSize(size int) bool      { return size > 0 && size <= MaxArtifactBytes }
func ValidApplicationDiscoverySchemeCount(count int) bool {
	return count >= MinApplicationDiscoveryAcceptedSchemes && count <= MaxApplicationDiscoveryAcceptedSchemes
}
func ValidApplicationDiscoveryTargetCount(count int) bool {
	return count >= MinApplicationDiscoveryTargets && count <= MaxApplicationDiscoveryTargets
}
func IsApplicationDiscoveryScheme(value string) bool {
	switch value {
	case ApplicationDiscoverySchemeHTTPS, ApplicationDiscoverySchemeHTTP, ApplicationDiscoverySchemeTCP:
		return true
	default:
		return false
	}
}

type ResourceContract struct {
	AllowEmptyID  bool
	OwnerRequired bool
}

func LookupResourceKind(kind string) (ResourceContract, bool) {
	if !ValidResourceKindSyntax(kind) {
		return ResourceContract{}, false
	}
	v, ok := resourceKinds[kind]
	return v, ok
}

func set(values ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	return m
}

var applicationActions = map[string]ApplicationActionContract{
	"application.content.put":       {Mutating: true},
	"application.content.get":       {},
	"application.discovery.resolve": {},
}
var operatorActions = set(
	"node.start", "node.stop", "node.status", "node.features", "node.runtime", "node.events", "config.effective", "config.reload", "transport.network_status", "transport.route_candidates",
	"discovery.status", "discovery.local_presence", "discovery.peers", "discovery.list_records", "discovery.resolve_record", "discovery.resolve_service", "discovery.import",
	"workload.register", "workload.start", "workload.stop", "workload.restart", "workload.status", "workload.list", "workload.hosted_service", "workload.service_publication", "workload.hosted_services",
	"data.publish_object", "data.get_object", "data.list_objects", "data.publish_blob", "data.get_blob", "data.fetch_blob", "data.retain_blob", "data.pin_blob", "data.drop_blob", "data.blob_sources", "data.list_blobs", "data.get_transfer", "data.list_transfers", "data.publish_manifest", "data.get_manifest", "data.list_manifests", "data.inventory",
	"diagnostics.snapshot", "diagnostics.health_summary", "diagnostics.pending_operations", "diagnostics.explain_failure", "diagnostics.recent_events",
	"realm.authority.create", "realm.channel.audit.read",
	"identity.principal.enroll", "identity.device.revoke", "identity.device-revocations.list", "identity.grant.issue", "identity.grant.revoke", "identity.grant.list")
var resourceKinds = map[string]ResourceContract{
	"node": {true, false}, "configuration": {true, false}, "network": {true, false}, "discovery-status": {true, false}, "local-presence": {true, false}, "peer-collection": {true, false},
	"discovery-record-collection": {true, false}, "discovery-record": {false, false}, "service": {false, false}, "workload": {false, false}, "workload-collection": {true, false},
	"service-collection": {true, false}, "content-object": {false, true}, "content-object-collection": {true, false}, "content-blob": {false, true}, "content-blob-collection": {true, false},
	"transfer": {false, false}, "transfer-collection": {true, false}, "content-manifest": {false, true}, "content-manifest-collection": {true, false}, "content-inventory": {true, false},
	"diagnostics": {true, false}, "operation-collection": {true, false}, "diagnostic-subject": {false, false}, "event-collection": {true, false}, "content-owner": {true, true}, "owned-content": {false, true},
	"service-type":             {false, false},
	"realm-authority-instance": {false, false}, "realm": {false, false},
	"principal": {false, false}, "device": {false, false}, "device-revocation-collection": {true, false}, "grant-proposal": {false, false}, "access-grant": {false, false}, "grant-collection": {true, false},
}
