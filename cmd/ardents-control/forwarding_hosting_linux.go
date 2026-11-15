package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
)

func runForwardingHostingPreparation(args []string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return &forwardingControlFailure{forwarding.ErrInput}
	}
	binding, policy, err := nodeHostingArguments(args, "forwarding", forwarding.ErrInput)
	if err != nil {
		return &forwardingControlFailure{err}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := forwarding.PrepareHosting(ctx, binding, policy); err != nil {
		return &forwardingControlFailure{err}
	}
	return writeSourceControlBytes(ctx, out, []byte("{\"operation\":\"forwarding.prepare-hosting\",\"prepared\":true,\"runtime_authority\":false}\n"))
}
