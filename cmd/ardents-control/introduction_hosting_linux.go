package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
)

func runIntroductionHostingPreparation(args []string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return &introductionControlFailure{introduction.ErrInput}
	}
	binding, policy, err := nodeHostingArguments(args, "introduction", introduction.ErrInput)
	if err != nil {
		return &introductionControlFailure{err}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := introduction.PrepareHosting(ctx, binding, policy); err != nil {
		return &introductionControlFailure{err}
	}
	return writeSourceControlBytes(ctx, out, []byte("{\"operation\":\"introduction.prepare-hosting\",\"prepared\":true,\"runtime_authority\":false}\n"))
}
