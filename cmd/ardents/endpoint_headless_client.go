package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
)

func runHeadlessOpen(ctx context.Context, socket, serviceLink, inputPath, outputPath string, output io.Writer) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer input.Close()
	result, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		_ = result.Close()
		if !keep {
			_ = os.Remove(outputPath)
		}
	}()
	application, err := applicationconnection.Dial(ctx, socket, serviceLink)
	if err != nil {
		return err
	}
	defer application.Close()
	response := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(result, application)
		response <- copyErr
	}()
	if _, err := io.Copy(application, input); err != nil {
		return err
	}
	if err := application.CloseInput(); err != nil {
		return err
	}
	if err := <-response; err != nil {
		return err
	}
	outcome, open := <-application.Done()
	if !open || outcome.Class == "" {
		return errors.New("headless Application ended without a terminal outcome")
	}
	if outcome.Class != "clean service connection close" {
		return errors.New(outcome.Class + ": " + outcome.Reason)
	}
	if err := result.Sync(); err != nil {
		return err
	}
	keep = true
	return json.NewEncoder(output).Encode(struct {
		Kind, Class, Reason string
	}{Kind: "headless-open-complete", Class: outcome.Class, Reason: outcome.Reason})
}

func runHeadlessAdministration(ctx context.Context, operation, socket string, output io.Writer) error {
	result, err := administration.Request(ctx, socket, administration.Operation(operation))
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]string{"kind": "headless-service-" + string(result)})
}
