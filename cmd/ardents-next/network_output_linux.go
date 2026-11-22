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
