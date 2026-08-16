package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/routeplan"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		if writeCommandError(os.Stderr, err) != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func writeCommandError(output io.Writer, err error) error {
	writeErr := json.NewEncoder(output).Encode(struct {
		Kind  string `json:"kind"`
		Error string `json:"error"`
	}{Kind: "error", Error: err.Error()})
	if writeErr != nil {
		return fmt.Errorf("write command error diagnostic: %w", writeErr)
	}
	return nil
}

func run(ctx context.Context, arguments []string, output io.Writer) (runErr error) {
	if len(arguments) != 2 && len(arguments) != 4 || arguments[0] != "run" || arguments[1] == "" ||
		len(arguments) == 4 && (arguments[2] != "--entry-plan" || arguments[3] == "") {
		return errors.New("usage: ardents-route run <role-plan.json> [--entry-plan <bridge-entry-plan.json>]")
	}
	sequence, err := routeplan.Load(arguments[1])
	if err != nil {
		return fmt.Errorf("load Route role plan: %w", err)
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	var entry *entryRuntime
	if len(arguments) == 4 {
		entry, err = loadEntryPlan(arguments[3])
		if err != nil {
			return fmt.Errorf("load bridge entry plan: %w", err)
		}
		defer func() { runErr = errors.Join(runErr, entry.close()) }()
	}
	if sequence.Concurrent() {
		if entry != nil {
			return errors.New("bridge entry plan is valid only for one client actor")
		}
		return routeplan.RunConcurrent(ctx, sequence, encoder.Encode)
	}
	for {
		step, ok, err := sequence.Next()
		if err != nil {
			return fmt.Errorf("construct Route Attachment: %w", err)
		}
		if !ok {
			return nil
		}
		if entry != nil {
			if step.Actor.Role != "client" || step.Actor.ManifestDigest != entry.manifest {
				_ = step.Close()
				return errors.New("bridge entry plan does not bind this client Route manifest")
			}
			step.Actor.OpenEntry = entry.open
		}
		var readyErr error
		var ready func(route.Evidence)
		if step.Actor.Role != "client" {
			ready = func(value route.Evidence) {
				value.Attachment = step.Attachment
				readyErr = encoder.Encode(value)
			}
		}
		result, runErr := route.Run(ctx, step.Actor, ready)
		runErr = errors.Join(runErr, readyErr)
		result.Attachment = step.Attachment
		closeErr := step.Close()
		if closeErr != nil {
			return errors.Join(runErr, fmt.Errorf("close Route Attachment %d: %w", step.Attachment, closeErr))
		}
		if runErr != nil && !step.More {
			result.Error = runErr.Error()
			return errors.Join(runErr, encoder.Encode(result))
		}
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("encode Route Attachment %d evidence: %w", step.Attachment, err)
		}
	}
}
