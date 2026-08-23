package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/routeplan"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
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
	if len(arguments) != 2 || arguments[0] != "run" || arguments[1] == "" {
		return errors.New("usage: ardents-route run <role-plan.json>")
	}
	sequence, err := routeplan.Load(arguments[1])
	if err != nil {
		return fmt.Errorf("load Route role plan: %w", err)
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return routeplan.Run(ctx, sequence, encoder.Encode, [32]byte{}, nil)
}
