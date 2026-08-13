package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/serviceendpoint"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) != 2 || arguments[0] != "run" || arguments[1] == "" {
		return errors.New("usage: ardents-service run <endpoint-plan.json>")
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	result, err := serviceendpoint.Run(ctx, arguments[1], func(role string) {
		_ = encoder.Encode(map[string]string{"kind": "ready", "role": role})
	})
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		return errors.Join(err, encodeErr)
	}
	return err
}
