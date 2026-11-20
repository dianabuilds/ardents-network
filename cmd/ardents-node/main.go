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
		if completedJoinExplicitStop(err) || completedDescriptorExplicitStop(err) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func runCommand(ctx context.Context, arguments []string, output io.Writer) error {
	// The retired Source/Forwarding orchestration and the retired legacy
	// Node/issuer/hosting paths refuse before any optional diagnostic socket is
	// opened. The maintained installed JOIN/Descriptor roles reach their own
	// admission first.
	if len(arguments) > 0 && (arguments[0] == "source" || arguments[0] == "installation" || arguments[0] == "forwarding" || arguments[0] == "join" || arguments[0] == "descriptor" || arguments[0] == "node" || arguments[0] == "issuer" || arguments[0] == "hosting") {
		return run(ctx, arguments, output)
	}
	return processdiag.Run(ctx, os.Getenv("ARDENTS_DEBUG_SOCKET"), func(workCtx context.Context) error { return run(workCtx, arguments, output) })
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) > 0 && arguments[0] == "descriptor" {
		return runDescriptorInstallation(ctx, arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "join" {
		return runJoinInstallation(ctx, arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "forwarding" {
		if len(arguments) == 3 && arguments[1] == "start-installed" {
			return errOldForwardingInstalledRetired
		}
		return errOldForwardingInstallationRetired
	}
	if len(arguments) > 0 && arguments[0] == "source" {
		if len(arguments) == 3 && arguments[1] == "--config" {
			return errOldSourceProfileRetired
		}
		if len(arguments) == 3 && arguments[1] == "start-installed" {
			return errOldSourceInstalledRetired
		}
		return errors.New("usage: ardents-node source start-installed BINDING")
	}
	if len(arguments) > 0 && arguments[0] == "installation" {
		return errOldSourceInstallationRetired
	}
	if len(arguments) > 0 && arguments[0] == "issuer" {
		return errOldIssuerRetired
	}
	if len(arguments) > 0 && arguments[0] == "hosting" {
		return errOldHostingRetired
	}
	if len(arguments) == 3 && arguments[0] == "node" && arguments[1] == "--config" {
		return errOldNodeConfigRetired
	}
	return errors.New("usage: ardents-node join ... | descriptor ...")
}
