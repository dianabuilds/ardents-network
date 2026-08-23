package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

// runEndpoint adapts one bounded Endpoint process to the retained command
// result projection. The Endpoint owns process and connection lifecycle; this
// command only selects its explicit operator route.
func runEndpoint(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) != 3 || arguments[1] != "run" || arguments[2] == "" {
		return errors.New("usage: ardents endpoint run <endpoint-plan.json>")
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	result, err := endpoint.Run(ctx, arguments[2], func(role string) {
		_ = encoder.Encode(map[string]string{"kind": "ready", "role": role})
	})
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		return errors.Join(err, encodeErr)
	}
	publishedAt := time.Now()
	publishErr := encoder.Encode(struct {
		Kind       string `json:"kind"`
		AtUnixNano int64  `json:"at_unix_nano"`
	}{Kind: "connection-result-published", AtUnixNano: publishedAt.UnixNano()})
	return errors.Join(err, publishErr)
}
