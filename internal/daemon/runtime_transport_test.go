package daemon

import (
	"context"
	"testing"

	networkwaku "ardents/internal/network/waku"

	"github.com/stretchr/testify/require"
)

func TestTransportProfilePayloadLockedClonesReducedCapabilities(t *testing.T) {
	svc := networkwaku.New()
	svc.SetBootstrapNodes([]string{"local://bootstrap"})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })

	mgr := &RuntimeManager{trans: svc}
	payload := mgr.transportProfilePayloadLocked()
	reduced := payload["reduced_capabilities"].([]string)
	if len(reduced) == 0 {
		t.Skip("transport profile did not report reduced capabilities")
	}

	snapshot := svc.ProfileSnapshot()
	reduced[0] = "mutated"
	require.NotEqual(t, "mutated", snapshot.ReducedCapabilities[0])
}
