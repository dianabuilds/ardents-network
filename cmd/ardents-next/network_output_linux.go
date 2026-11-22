package main

import (
	"context"
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

type networkOutputFile struct {
	name string
	raw  []byte
}

func persistNetworkFiles(ctx context.Context, path string, files []networkOutputFile, secret bool, check func() error, observe func(string, *os.File)) error {
	inventory := make([]artifacts.Content, 0, len(files))
	for _, file := range files {
		if len(file.raw) > maximumNetworkPreparationPlan {
			return errNetworkSigning
		}
		inventory = append(inventory, artifacts.Content{Name: file.name, Data: file.raw})
	}
	if err := artifacts.PublishInventory(ctx, path, inventory, secret, check, observe); err != nil {
		return errors.Join(errNetworkSigning, err)
	}
	return nil
}

func writeNetworkPreparation(ctx context.Context, path, name string, raw []byte) error {
	return persistNetworkPreparation(ctx, path, name, raw, nil)
}

func persistNetworkPreparation(ctx context.Context, path, name string, raw []byte, observe func(string, *os.File)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(raw) == 0 || len(raw) > maximumNetworkPreparationPlan ||
		(name != "node-record.signing-input" && name != "epoch.unsigned" && name != "profile.unsigned") {
		return errNetworkPreparation
	}
	if err := artifacts.PublishUnsigned(ctx, path, artifacts.Content{Name: name, Data: raw}, observe); err != nil {
		return errors.Join(errNetworkPreparation, err)
	}
	return nil
}
