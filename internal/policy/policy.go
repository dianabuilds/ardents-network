package policy

import (
	"ardents/internal/content"
	"time"

	identityapi "ardents/internal/identity"
	transport "ardents/internal/network"
	"ardents/internal/workload/execution"
	domainworkload "ardents/internal/workload/registry"
)

type Snapshot struct {
	State  string `json:"state,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type Policy interface {
	AdmitWorkload(domainworkload.Spec, []execution.Status) error
	AllowServicePublication(domainworkload.ServiceSpec) error
	AllowBlobRetention(content.BlobPolicyView, bool, time.Time, time.Time) error
	AllowBlobPin(content.BlobPolicyView) error
	AllowPeerBlobReserving(content.BlobPolicyView) error
	AllowReplicaBlobServing(content.BlobPolicyView) error
	AllowRouteUse(transport.Candidate) error
	AllowCapabilityUse(identityapi.CapabilityUse) error
	Snapshot() Snapshot
}
