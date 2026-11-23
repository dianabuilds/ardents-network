package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

var errDocumentUsage = errors.New("invalid document command arguments")

func runDocumentCommand(ctx context.Context, arguments []string, diagnostic io.Writer) int {
	if err := executeDocumentCommand(ctx, arguments); err != nil {
		fmt.Fprintln(diagnostic, documentFailure(err))
		return 2
	}
	return 0
}

func executeDocumentCommand(ctx context.Context, arguments []string) error {
	switch {
	case len(arguments) == 2 && arguments[0] == "link" && filepath.IsAbs(arguments[1]):
		return runPublishedLink(ctx, arguments[1])
	case len(arguments) == 3 && arguments[0] == "publish" && filepath.IsAbs(arguments[1]) && filepath.IsAbs(arguments[2]):
		return publishDocument(ctx, arguments[1], arguments[2])
	case len(arguments) == 2 && arguments[0] == "read" && filepath.IsAbs(arguments[1]):
		input, output, err := openDocumentCommandIO()
		if err != nil {
			return err
		}
		return readDocument(ctx, arguments[1], input, output)
	default:
		return errDocumentUsage
	}
}

func documentFailure(err error) string {
	switch {
	case errors.Is(err, errDocumentUsage):
		return documentUsage
	case errors.Is(err, context.Canceled):
		return "read cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "read timed out"
	case errors.Is(err, errDocumentInput):
		return "document destination or local input is invalid"
	}
	if refusal, ok := errors.AsType[connection.SetupRefusalError](err); ok {
		switch refusal.Outcome().Class {
		case connection.LocalCancellation:
			return "read cancelled"
		case connection.LocalTimeout:
			return "read timed out"
		case connection.LocalFailure:
			return "local connection failed"
		case connection.IndeterminateFailure:
			return "connection outcome unknown"
		case connection.CapacityUnavailable:
			return "document capacity unavailable"
		}
	}
	return "document operation unavailable"
}
