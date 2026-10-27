package main

import (
	"context"
	"flag"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
)

func forwardingHostingArguments(args []string) (string, hosting.Policy, error) {
	var binding, from, until, interfaces string
	var policy hosting.Policy
	if len(args) > 24 {
		return "", policy, forwarding.ErrInput
	}
	flags := flag.NewFlagSet("forwarding prepare-hosting", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&binding, "binding", "", "original completed Forwarding binding")
	flags.StringVar(&policy.Provider, "provider", "", "declared provider")
	flags.StringVar(&from, "start", "", "original period start, RFC3339 whole seconds")
	flags.StringVar(&until, "end", "", "original period end, RFC3339 whole seconds")
	flags.StringVar(&policy.Unit, "unit", "", "provider denomination")
	flags.Uint64Var(&policy.Quantity, "quantity", 0, "original provider allowance")
	flags.StringVar(&policy.Directions, "directions", "", "tx, rx or tx+rx")
	flags.StringVar(&interfaces, "interfaces", "", "comma-separated actual interface names")
	flags.Uint64Var(&policy.InitialUsedBytes, "initial-used-bytes", 0, "already consumed floor")
	flags.Uint64Var(&policy.LowWatermarkBytes, "low-watermark-bytes", 0, "retained low watermark")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 10 || !forwardingUniqueOptions(args, "") || !sourceControlPath(binding) {
		return "", policy, forwarding.ErrInput
	}
	var err error
	policy.Start, err = time.Parse(time.RFC3339, from)
	if err != nil || policy.Start.Format(time.RFC3339) != from {
		return "", policy, forwarding.ErrInput
	}
	policy.End, err = time.Parse(time.RFC3339, until)
	if err != nil || policy.End.Format(time.RFC3339) != until {
		return "", policy, forwarding.ErrInput
	}
	policy.Interfaces = strings.Split(interfaces, ",")
	if err := policy.Validate(); err != nil {
		return "", policy, err
	}
	return binding, policy, nil
}

func runForwardingHostingPreparation(args []string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return &forwardingControlFailure{forwarding.ErrInput}
	}
	binding, policy, err := forwardingHostingArguments(args)
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
