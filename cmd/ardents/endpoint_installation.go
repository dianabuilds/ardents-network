package main

import (
	"context"
	"encoding/json"
	"errors"
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

func runEndpointProvision(ctx context.Context, path string, output io.Writer) error {
	result, err := installation.Provision(ctx, path)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func runEndpointUpgrade(ctx context.Context, path string, output io.Writer) error {
	result, err := installation.Upgrade(ctx, path)
	if err != nil {
		if result.Status != "" {
			return errors.Join(err, json.NewEncoder(output).Encode(result))
		}
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func runEndpointRecovery(ctx context.Context, root string, output io.Writer) error {
	result, err := installation.Recover(ctx, root)
	if err != nil {
		if result.Status != "" {
			return errors.Join(err, json.NewEncoder(output).Encode(result))
		}
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func runInstalledEndpoint(ctx context.Context, root string, output io.Writer) error {
	plan, err := installation.AdmitStart(ctx, root)
	if err != nil {
		return err
	}
	return runTextHeadlessRuntime(ctx, plan, output)
}
