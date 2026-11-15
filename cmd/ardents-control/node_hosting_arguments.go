package main

import (
	"flag"
	"io"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

func nodeHostingArguments(args []string, role string, invalid error) (string, hosting.Policy, error) {
	var binding, from, until, interfaces string
	var policy hosting.Policy
	if len(args) > 24 {
		return "", policy, invalid
	}
	flags := flag.NewFlagSet(role+" prepare-hosting", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&binding, "binding", "", "original completed Node binding")
	flags.StringVar(&policy.Provider, "provider", "", "declared provider")
	flags.StringVar(&from, "start", "", "original period start, RFC3339 whole seconds")
	flags.StringVar(&until, "end", "", "original period end, RFC3339 whole seconds")
	flags.StringVar(&policy.Unit, "unit", "", "provider denomination")
	flags.Uint64Var(&policy.Quantity, "quantity", 0, "original provider allowance")
	flags.StringVar(&policy.Directions, "directions", "", "tx, rx or tx+rx")
	flags.StringVar(&interfaces, "interfaces", "", "comma-separated actual interface names")
	flags.Uint64Var(&policy.InitialUsedBytes, "initial-used-bytes", 0, "already consumed floor")
	flags.Uint64Var(&policy.LowWatermarkBytes, "low-watermark-bytes", 0, "retained low watermark")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 10 || !uniqueNodeOptions(args, "") || !sourceControlPath(binding) {
		return "", policy, invalid
	}
	var err error
	policy.Start, err = time.Parse(time.RFC3339, from)
	if err != nil || policy.Start.Format(time.RFC3339) != from {
		return "", policy, invalid
	}
	policy.End, err = time.Parse(time.RFC3339, until)
	if err != nil || policy.End.Format(time.RFC3339) != until {
		return "", policy, invalid
	}
	policy.Interfaces = strings.Split(interfaces, ",")
	if err := policy.Validate(); err != nil {
		return "", policy, err
	}
	return binding, policy, nil
}
