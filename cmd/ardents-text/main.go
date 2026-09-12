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

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if os.Getenv("ARDENTS_TEXT_WORKER_TEST_DIAGNOSTIC") == "1" {
			fmt.Fprintln(os.Stdout, err)
		}
		// Ordinary diagnostics never include document, destination or worker input.
		fmt.Fprintln(os.Stderr, textFailure(err))
		os.Exit(2)
	}
}

func run(arguments []string) error {
	if len(arguments) == 2 && arguments[0] == "link" && filepath.IsAbs(arguments[1]) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runPublishedLink(ctx, arguments[1])
	}
	if len(arguments) == 3 && arguments[0] == "publish" && filepath.IsAbs(arguments[1]) && filepath.IsAbs(arguments[2]) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return publishText(ctx, arguments[1], arguments[2])
	}
	if len(arguments) == 2 && arguments[0] == "read" && filepath.IsAbs(arguments[1]) {
		// Signal handling begins only in the trusted UI. The fixed worker must
		// reach its inherited-descriptor audit without UI initialization.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		input, output, err := openTextCommandIO()
		if err != nil {
			return err
		}
		return readText(ctx, arguments[1], input, output)
	}
	if len(arguments) != 1 {
		return errors.New("text worker entrypoint is invalid")
	}
	switch arguments[0] {
	case "worker-reader":
		return textdocument.RunInheritedWorker(textdocument.ReaderWorker)
	case "worker-publisher":
		return textdocument.RunInheritedWorker(textdocument.PublisherWorker)
	default:
		return errors.New("text worker entrypoint is invalid")
	}
}

func textFailure(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "text read cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "text read timed out"
	case errors.Is(err, errTextInput):
		return "text destination or local input is invalid"
	default:
		return "text operation unavailable"
	}
}
