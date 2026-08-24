package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/endpoint/portable"
)

// runEndpoint adapts one bounded Endpoint process to the retained command
// result projection. The Endpoint owns process and connection lifecycle; this
// command only selects its explicit operator route.
func runEndpoint(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) == 2 && arguments[1] == "portable" {
		return runPortableEndpoint(ctx, output)
	}
	if len(arguments) != 3 || arguments[1] != "run" || arguments[2] == "" {
		return errors.New("usage: ardents endpoint <portable|run <endpoint-plan.json>>")
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

// runPortableEndpoint adapts the selected H4-1A local lifecycle to the
// command's bounded event projection. It intentionally creates no network
// route, browser integration, or local application capability.
func runPortableEndpoint(ctx context.Context, output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	config, err := portable.DefaultConfig()
	if err != nil {
		_ = encoder.Encode(portableEvent(portable.Event{State: portable.StateStarting}))
		_ = encoder.Encode(portableEvent(portable.Event{State: portable.StateIncompatible, Reason: portable.ReasonLocalProfileInvalid}))
		return err
	}
	return portable.Run(ctx, config, func(event portable.Event) {
		_ = encoder.Encode(portableEvent(event))
	})
}

func portableEvent(event portable.Event) struct {
	Kind       string `json:"kind"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
	Attachment string `json:"attachment,omitempty"`
} {
	return struct {
		Kind       string `json:"kind"`
		State      string `json:"state"`
		Reason     string `json:"reason,omitempty"`
		Attachment string `json:"attachment,omitempty"`
	}{Kind: "endpoint-lifecycle", State: string(event.State), Reason: string(event.Reason), Attachment: event.Attachment}
}
