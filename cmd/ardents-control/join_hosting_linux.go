package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/join"
)

func runJoinHostingPreparation(args []string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return &joinControlFailure{join.ErrInput}
	}
	binding, policy, err := nodeHostingArguments(args, "join", join.ErrInput)
	if err != nil {
		return &joinControlFailure{err}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := join.PrepareHosting(ctx, binding, policy); err != nil {
		return &joinControlFailure{err}
	}
	return writeSourceControlBytes(ctx, out, []byte("{\"operation\":\"join.prepare-hosting\",\"prepared\":true,\"runtime_authority\":false}\n"))
}
