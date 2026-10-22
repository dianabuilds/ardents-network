package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	processdiag "github.com/dianabuilds/ardents-network/internal/diagnostics/process"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), nodeTerminationSignals()...)
	defer stop()
	arguments := os.Args[1:]
	if err := runCommand(ctx, arguments, os.Stdout); err != nil {
		if completedExplicitSourceStop(err) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func runCommand(ctx context.Context, arguments []string, output io.Writer) error {
	// Installed Source and its retired selector reach their own admission before
	// any optional diagnostic socket is opened. Installation is a distinct root
	// operation, with no process diagnostic side effect either.
	if len(arguments) > 0 && (arguments[0] == "source" || arguments[0] == "installation") {
		return run(ctx, arguments, output)
	}
	return processdiag.Run(ctx, os.Getenv("ARDENTS_DEBUG_SOCKET"), func(workCtx context.Context) error { return run(workCtx, arguments, output) })
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) > 0 && arguments[0] == "source" {
		if len(arguments) == 3 && arguments[1] == "--config" {
			return errOldSourceProfileRetired
		}
		if len(arguments) == 3 && arguments[1] == "start-installed" {
			return runInstalledSource(ctx, arguments[2], output)
		}
		return errors.New("usage: ardents-node source start-installed BINDING")
	}
	if len(arguments) > 0 && arguments[0] == "installation" {
		return runSourceInstallation(ctx, arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "issuer" {
		return runIssuer(ctx, arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "hosting" {
		return runHosting(arguments[1:])
	}
	if len(arguments) != 3 || arguments[1] != "--config" {
		return errors.New("usage: ardents-node (source|node) --config PATH | issuer (initialize|serve) --config PATH | hosting initialize --config PATH")
	}
	if arguments[0] == "node" {
		return runNode(ctx, arguments[2], output)
	}
	return errors.New("usage: ardents-node source start-installed BINDING | node --config PATH | issuer (initialize|serve) --config PATH | hosting initialize --config PATH")
}
