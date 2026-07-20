package lifecycle

import (
	"context"
	"testing"

	transport "ardents/internal/network/api"

	"github.com/stretchr/testify/require"
)

func TestTransportProfilePayloadLockedClonesReducedCapabilities(t *testing.T) {
	svc := transport.New()
	svc.SetBootstrapNodes([]string{"local://bootstrap"})
	_ = svc.Start(context.Background())

	mgr := &Manager{trans: svc}
	payload := mgr.transportProfilePayloadLocked()
	reduced := payload["reduced_capabilities"].([]string)
	if len(reduced) == 0 {
		t.Skip("transport profile did not report reduced capabilities")
	}

	snapshot := svc.ProfileSnapshot()
	reduced[0] = "mutated"
	require.NotEqual(t, "mutated", snapshot.ReducedCapabilities[0])
}
