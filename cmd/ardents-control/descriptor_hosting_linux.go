package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor"
)

func runDescriptorHostingPreparation(args []string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return &descriptorControlFailure{descriptor.ErrInput}
	}
	binding, policy, err := nodeHostingArguments(args, "descriptor", descriptor.ErrInput)
	if err != nil {
		return &descriptorControlFailure{err}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := descriptor.PrepareHosting(ctx, binding, policy); err != nil {
		return &descriptorControlFailure{err}
	}
	return writeSourceControlBytes(ctx, out, []byte("{\"operation\":\"descriptor.prepare-hosting\",\"prepared\":true,\"runtime_authority\":false}\n"))
}
