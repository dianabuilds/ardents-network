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
	"github.com/dianabuilds/ardents-network/internal/successor/installation/join"
)

func joinHostingArguments(args []string) (string, hosting.Policy, error) {
	var binding, from, until, interfaces string
	var policy hosting.Policy
	if len(args) > 24 {
		return "", policy, join.ErrInput
	}
	flags := flag.NewFlagSet("join prepare-hosting", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&binding, "binding", "", "original completed Join binding")
	flags.StringVar(&policy.Provider, "provider", "", "declared provider")
	flags.StringVar(&from, "start", "", "original period start, RFC3339 whole seconds")
	flags.StringVar(&until, "end", "", "original period end, RFC3339 whole seconds")
	flags.StringVar(&policy.Unit, "unit", "", "provider denomination")
	flags.Uint64Var(&policy.Quantity, "quantity", 0, "original provider allowance")
	flags.StringVar(&policy.Directions, "directions", "", "tx, rx or tx+rx")
	flags.StringVar(&interfaces, "interfaces", "", "comma-separated actual interface names")
	flags.Uint64Var(&policy.InitialUsedBytes, "initial-used-bytes", 0, "already consumed floor")
	flags.Uint64Var(&policy.LowWatermarkBytes, "low-watermark-bytes", 0, "retained low watermark")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 10 || !joinUniqueOptions(args, "") || !sourceControlPath(binding) {
		return "", policy, join.ErrInput
	}
	var err error
	policy.Start, err = time.Parse(time.RFC3339, from)
	if err != nil || policy.Start.Format(time.RFC3339) != from {
		return "", policy, join.ErrInput
	}
	policy.End, err = time.Parse(time.RFC3339, until)
	if err != nil || policy.End.Format(time.RFC3339) != until {
		return "", policy, join.ErrInput
	}
	policy.Interfaces = strings.Split(interfaces, ",")
	if err := policy.Validate(); err != nil {
		return "", policy, err
	}
	return binding, policy, nil
}

func runJoinHostingPreparation(args []string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return &joinControlFailure{join.ErrInput}
	}
	binding, policy, err := joinHostingArguments(args)
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

// Public flags cannot repeat a purpose-bound declaration under another spelling.
func joinUniqueOptions(args []string, repeatable string) bool {
	seen := map[string]bool{}
	for _, argument := range args {
		if !strings.HasPrefix(argument, "-") {
			continue
		}
		name := strings.TrimLeft(argument, "-")
		name, _, _ = strings.Cut(name, "=")
		if seen[name] && name != repeatable {
			return false
		}
		seen[name] = true
	}
	return true
}
