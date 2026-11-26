package networksigning

import (
	"context"
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// publishInventory holds finite output through the original custody check.
func publishInventory(ctx context.Context, path string, files []artifacts.Content, secret bool, check func() error, observe func(string, *os.File)) error {
	for _, file := range files {
		if len(file.Data) > networkdraft.MaximumPlanBytes {
			return errNetworkSigning
		}
	}
	if err := artifacts.PublishInventory(ctx, path, files, secret, check, observe); err != nil {
		return errors.Join(errNetworkSigning, err)
	}
	return nil
}
