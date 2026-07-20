package recovery

import (
	"context"
	"crypto/ed25519"

	domainworkload "ardents/internal/workload/workload"
)

func PublishDiscoveryForStartup(
	ctx context.Context,
	_ ed25519.PrivateKey,
	refreshNetworkPublication func(context.Context) error,
	bootstrapDiscovery func(context.Context) error,
) error {
	if err := refreshNetworkPublication(ctx); err != nil {
		return err
	}
	return bootstrapDiscovery(ctx)
}

func StartWorkloadsForStartup(
	ctx context.Context,
	workloadSpecs []domainworkload.Spec,
	seedWorkloads func(context.Context, []domainworkload.Spec) error,
) error {
	return seedWorkloads(ctx, append([]domainworkload.Spec(nil), workloadSpecs...))
}
