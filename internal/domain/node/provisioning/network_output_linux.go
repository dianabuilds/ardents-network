package provisioning

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"os"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// PublishNetworkInventory holds finite output through the original custody check.
func PublishNetworkInventory(ctx context.Context, path string, files []artifacts.Content, secret bool, check func() error, observe func(string, *os.File)) error {
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

// PublishPreparedNetworkArtifact writes one of the three fixed unsigned artifacts.
func PublishPreparedNetworkArtifact(ctx context.Context, path, name string, raw []byte, observe func(string, *os.File)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(raw) == 0 || len(raw) > networkdraft.MaximumPlanBytes ||
		(name != "node-record.signing-input" && name != "epoch.unsigned" && name != "profile.unsigned") {
		return networkdraft.ErrUnavailable
	}
	if err := artifacts.PublishUnsigned(ctx, path, artifacts.Content{Name: name, Data: raw}, observe); err != nil {
		return errors.Join(networkdraft.ErrUnavailable, err)
	}
	return nil
}
