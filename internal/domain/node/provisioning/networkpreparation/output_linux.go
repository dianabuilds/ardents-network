package networkpreparation

import (
	"context"
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// publish writes one of the three fixed unsigned artifacts.
func publish(ctx context.Context, path, name string, raw []byte, observe func(string, *os.File)) error {
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
