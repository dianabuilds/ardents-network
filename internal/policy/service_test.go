package policy

import (
	"testing"
	"time"

	dataapi "ardents/internal/data/api"
	hostingservice "ardents/internal/hosting/service"
	transport "ardents/internal/network/api"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"

	"github.com/stretchr/testify/require"
)

func TestPolicyAllowDenyMatrix(t *testing.T) {
	svc := New(Config{
		MaxWorkloads:                    1,
		DeniedCapabilities:              []string{"gpu"},
		DisableNetworkPublishedServices: true,
		DeniedRouteSchemes:              []string{"quic"},
		DisablePeerBlobReserving:        true,
		MaxLocalRetentionTTL:            time.Hour,
	})

	existing := []observedstate.Status{{Spec: domainworkload.Spec{ID: "work.present"}}}
	{
		err := svc.AdmitWorkload(domainworkload.Spec{ID: "work.new", Kind: "service", Desired: "running"}, existing)
		require.Error(t, err, "expected workload limit denial")
	}
	{
		err := svc.AdmitWorkload(domainworkload.Spec{ID: "work.gpu", Kind: "service", Capabilities: []string{"gpu"}}, nil)
		require.Error(t, err, "expected denied capability")
	}
	{
		err := svc.AdmitWorkload(domainworkload.Spec{ID: "work.unlisted", Kind: "service", PolicyRef: "trusted"}, nil)
		require.Error(t, err, "expected unlisted policy reference denial")
	}
	{
		err := svc.AllowServicePublication(hostingservice.Spec{ID: "svc.echo", Type: "echo", Mode: "NetworkPublished"})
		require.Error(t, err, "expected network publication denial")
	}
	{
		err := svc.AllowRouteUse(transport.Candidate{Scheme: "quic", Trusted: true})
		require.Error(t, err, "expected denied route scheme")
	}
	{
		err := svc.AllowPeerBlobReserving(dataapi.BlobSnapshot{ID: "blob", State: "available-local", Retention: "pinned", Encrypted: true})
		require.Error(t, err, "expected peer re-serving denial")
	}
	{
		err := svc.AllowReplicaBlobServing(dataapi.BlobSnapshot{ID: "blob", State: "retained-temporary", Retention: "relay-temporary", Encrypted: true})
		require.Error(t, err, "expected committed replica serving denial when peer serving is disabled")
	}
	{
		err := svc.AllowBlobRetention(dataapi.BlobSnapshot{ID: "blob", Encrypted: true}, false, time.Now().Add(2*time.Hour), time.Now())
		require.Error(t, err, "expected local retention ttl denial")
	}

	allowed := New(Config{
		AllowedPolicyRefs:          []string{"trusted"},
		AllowReservingRelayBlobs:   true,
		AllowPinRelayRetainedBlobs: true,
	})
	{
		err := allowed.AdmitWorkload(domainworkload.Spec{ID: "work.ok", Kind: "service", PolicyRef: "trusted"}, nil)
		require.NoErrorf(t, err, "admit allowed workload: %v", err)
	}
	{
		err := allowed.AllowServicePublication(hostingservice.Spec{ID: "svc.ok", Type: "echo", Mode: "LocalOnly"})
		require.NoErrorf(t, err, "allow local service publication: %v", err)
	}
	{
		err := allowed.AllowRouteUse(transport.Candidate{Scheme: "tcp", Trusted: true})
		require.NoErrorf(t, err, "allow trusted tcp route: %v", err)
	}
	{
		err := allowed.AllowPeerBlobReserving(dataapi.BlobSnapshot{ID: "blob", State: "available-local", Retention: "relay-temporary", Encrypted: true})
		require.NoErrorf(t, err, "allow relay blob reserving: %v", err)
	}
	{
		defaultPolicy := New(Config{})
		err := defaultPolicy.AllowReplicaBlobServing(dataapi.BlobSnapshot{ID: "replica", State: "retained-temporary", Retention: "relay-temporary", Encrypted: true})
		require.NoErrorf(t, err, "allow current committed replica serving: %v", err)
	}
}

func TestPolicySnapshotStaysEnforcedAfterSubsequentAllows(t *testing.T) {
	svc := New(Config{
		DeniedCapabilities: []string{"gpu"},
	})
	{
		err := svc.AdmitWorkload(domainworkload.Spec{ID: "work.gpu", Kind: "service", Capabilities: []string{"gpu"}}, nil)
		require.Error(t, err, "expected denied capability")
	}
	{
		snapshot := svc.Snapshot()
		require.Falsef(t, snapshot.State !=
			"enforced", "snapshot state = %q, want enforced", snapshot.State)
	}
	{
		err := svc.AllowBlobPin(dataapi.BlobSnapshot{ID: "blob", State: "available-local"})
		require.NoErrorf(t, err, "allow blob pin: %v", err)
	}
	{
		snapshot := svc.Snapshot()
		require.Falsef(t, snapshot.State !=
			"enforced", "snapshot state after allow = %q, want enforced", snapshot.State)
	}
	{
		snapshot := svc.Snapshot()
		require.False(t, snapshot.Reason ==
			"", "expected deny reason to remain visible")
	}
}

func TestPolicyUsesOneNormalizationRuleAcrossSurfaces(t *testing.T) {
	svc := New(Config{
		AllowedPolicyRefs:  []string{" Trusted ", "trusted"},
		DeniedCapabilities: []string{" GPU "},
		DeniedServiceTypes: []string{" Admin "},
		DeniedRouteSchemes: []string{" Quic "},
	})
	{
		err := svc.AdmitWorkload(domainworkload.Spec{ID: "work.ok", Kind: "service", PolicyRef: "trusted"}, nil)
		require.NoErrorf(t, err, "admit normalized policy ref: %v", err)
	}
	{
		err := svc.AdmitWorkload(domainworkload.Spec{ID: "work.gpu", Kind: "service", Capabilities: []string{"gpu"}}, nil)
		require.Error(t, err, "expected normalized denied capability")
	}
	{
		err := svc.AllowServicePublication(hostingservice.Spec{ID: "svc.admin", Type: "admin", Mode: "LocalOnly"})
		require.Error(t, err, "expected normalized denied service type")
	}
	{
		err := svc.AllowRouteUse(transport.Candidate{Scheme: "quic", Trusted: true})
		require.Error(t, err, "expected normalized denied route scheme")
	}
}
