package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/endpoint/installation"
)

func runInstallationCheck(ctx context.Context, root string, output io.Writer) error {
	observation, err := installation.Check(ctx, root)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(observation)
}
