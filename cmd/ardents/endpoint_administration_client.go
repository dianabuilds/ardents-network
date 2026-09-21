package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
)

func runHeadlessAdministration(ctx context.Context, operation, socket string, output io.Writer) error {
	result, err := administration.Request(ctx, socket, administration.Operation(operation))
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]string{"kind": "headless-service-" + string(result)})
}
