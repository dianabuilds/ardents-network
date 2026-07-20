package api

import (
	"time"

	dataapi "ardents/internal/data/api"
	hostingservice "ardents/internal/hosting/service"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	nodeapi "ardents/internal/node/api"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

type Service interface {
	AdmitWorkload(domainworkload.Spec, []observedstate.Status) error
	AllowServicePublication(hostingservice.Spec) error
	AllowBlobRetention(dataapi.BlobSnapshot, bool, time.Time, time.Time) error
	AllowBlobPin(dataapi.BlobSnapshot) error
	AllowPeerBlobReserving(dataapi.BlobSnapshot) error
	AllowReplicaBlobServing(dataapi.BlobSnapshot) error
	AllowRouteUse(transport.Candidate) error
	AllowCapabilityUse(identityapi.CapabilityUse) error
	Snapshot() nodeapi.PartSnapshot
}
