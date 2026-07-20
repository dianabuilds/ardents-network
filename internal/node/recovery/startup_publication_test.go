package recovery_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"

	noderecovery "ardents/internal/node/recovery"
	domainworkload "ardents/internal/workload/workload"
	"github.com/stretchr/testify/require"
)

func TestPublishDiscoveryForStartupStopsOnRefreshFailure(t *testing.T) {
	calledBootstrap := false

	err := noderecovery.PublishDiscoveryForStartup(
		context.Background(),
		ed25519.PrivateKey("private"),
		func(context.Context) error { return errors.New("refresh failed") },
		func(context.Context) error {
			calledBootstrap = true
			return nil
		},
	)

	require.Error(t, err)
	require.False(t, calledBootstrap)
}

func TestStartWorkloadsForStartupClonesSpecs(t *testing.T) {
	specs := []domainworkload.Spec{{ID: "work-1"}}
	captured := []domainworkload.Spec{}

	err := noderecovery.StartWorkloadsForStartup(
		context.Background(),
		specs,
		func(_ context.Context, in []domainworkload.Spec) error {
			captured = append([]domainworkload.Spec(nil), in...)
			return nil
		},
	)

	require.NoError(t, err)
	captured[0].ID = "mutated"
	require.Equal(t, "work-1", specs[0].ID)
}
