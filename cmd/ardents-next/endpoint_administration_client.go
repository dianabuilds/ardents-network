package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
)

func withdrawEndpoint(ctx context.Context, socket string, output io.Writer) error {
	result, err := administration.Request(ctx, socket, administration.Withdraw)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]string{"kind": "headless-service-" + string(result)})
}
