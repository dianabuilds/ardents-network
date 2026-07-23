package daemon

import (
	"ardents/internal/network"
	"context"
	"testing"

	networkwaku "ardents/internal/network/waku"

	"github.com/stretchr/testify/require"
)

func TestTransportProfilePayloadLockedClonesReducedFeatures(t *testing.T) {
	svc := networkwaku.New()
	svc.SetBootstrapNodes([]string{"local://bootstrap"})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })

	mgr := &RuntimeManager{trans: svc}
	payload := mgr.transportProfilePayloadLocked()
	reduced := payload["reduced_features"].([]network.TransportFeature)
	if len(reduced) == 0 {
		t.Skip("transport profile did not report reduced features")
	}

	snapshot := svc.ProfileSnapshot()
	reduced[0] = network.TransportFeature("mutated")
	require.NotEqual(t, network.TransportFeature("mutated"), snapshot.ReducedFeatures[0])
}
