// The fixed worker must not open host cgroup files before its descriptor audit.
// Its installed unit supplies a fixed GOMAXPROCS and enforces the resource cap.
//
//go:debug containermaxprocs=0
//go:debug updatemaxprocs=0
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

var errTextUsage = errors.New("usage: ardents-text publish <absolute-administration-socket> <absolute-document-file> | link <absolute-administration-socket> | read <absolute-application-socket>\nPublication succeeds only after Descriptor acknowledgement; link and withdrawal use separate Administration authorization. Withdraw with ardents endpoint withdraw <administration-socket>")

func main() {
	if err := run(os.Args[1:]); err != nil {
		// Ordinary diagnostics never include document, destination or worker input.
		fmt.Fprintln(os.Stderr, textFailure(err))
		os.Exit(2)
	}
}

func run(arguments []string) error {
	if len(arguments) == 1 {
		// The fixed entrypoints audit inherited descriptors before trusted UI
		// signal handling or stream setup.
		switch arguments[0] {
		case "worker-reader":
			return textdocument.RunInheritedWorker(textdocument.ReaderWorker)
		case "worker-publisher":
			return textdocument.RunInheritedWorker(textdocument.PublisherWorker)
		case "help", "--help":
			return errTextUsage
		}
	}
	if len(arguments) == 0 {
		return errTextUsage
	}
	var operation func(context.Context) error
	switch {
	case len(arguments) == 2 && arguments[0] == "link" && filepath.IsAbs(arguments[1]):
		operation = func(ctx context.Context) error { return runPublishedLink(ctx, arguments[1]) }
	case len(arguments) == 3 && arguments[0] == "publish" && filepath.IsAbs(arguments[1]) && filepath.IsAbs(arguments[2]):
		operation = func(ctx context.Context) error { return publishText(ctx, arguments[1], arguments[2]) }
	case len(arguments) == 2 && arguments[0] == "read" && filepath.IsAbs(arguments[1]):
		operation = func(ctx context.Context) error {
			input, output, err := openTextCommandIO()
			if err != nil {
				return err
			}
			return readText(ctx, arguments[1], input, output)
		}
	default:
		return errors.New("text worker entrypoint is invalid")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return operation(ctx)
}

func textFailure(err error) string {
	switch {
	case errors.Is(err, errTextUsage):
		return errTextUsage.Error()
	case errors.Is(err, context.Canceled):
		return "text read cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "text read timed out"
	case errors.Is(err, errTextInput):
		return "text destination or local input is invalid"
	}
	var refusal connection.SetupRefusalError
	if errors.As(err, &refusal) {
		switch refusal.Outcome().Class {
		case connection.LocalCancellation:
			return "text read cancelled"
		case connection.LocalTimeout:
			return "text read timed out"
		case connection.LocalFailure:
			return "text local connection failed"
		case connection.IndeterminateFailure:
			return "text connection outcome unknown"
		case connection.CapacityUnavailable:
			return "text capacity unavailable"
		}
	}
	return "text operation unavailable"
}
