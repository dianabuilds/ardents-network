package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) != 2 || arguments[0] != "run" || arguments[1] == "" {
		return errors.New("usage: ardents-route run <role-plan.json>")
	}
	raw, err := readActorPlan(arguments[1])
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	attempts := raw.Attachments
	if attempts == 0 {
		attempts = 1
	}
	for attempt := uint32(0); attempt < attempts; attempt++ {
		if attempt > 0 {
			raw.AcknowledgementSocket, raw.AcknowledgementKey = "", ""
		}
		actor, closeState, err := raw.actor()
		if err != nil {
			return err
		}
		var ready func(route.Evidence)
		if actor.Role != "client" {
			ready = func(value route.Evidence) { _ = encoder.Encode(value) }
		}
		result, runErr := route.Run(ctx, actor, ready)
		result.Attachment = attempt + 1
		if closeState != nil {
			closeState()
		}
		if runErr != nil && attempt+1 == attempts {
			result.Error = runErr.Error()
			_ = encoder.Encode(result)
			return runErr
		}
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}
	return nil
}
