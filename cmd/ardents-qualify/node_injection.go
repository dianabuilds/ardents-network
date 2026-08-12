package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"time"

	nodequalification "github.com/dianabuilds/ardents-network/internal/qualification/node"
	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func evaluateNodeInjection(arguments []string) (statequalification.Result, error) {
	flags := flag.NewFlagSet("inject-node", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var addresses, secrets, plan, mode string
	flags.StringVar(&mode, "mode", "probe", "probe, memory, cpu, nofile, or emfile")
	flags.StringVar(&addresses, "addresses", "", "comma-separated literal Node addresses")
	flags.StringVar(&secrets, "secrets", "", "isolated Harness credential root")
	flags.StringVar(&plan, "plan", "", "frozen public role-probe plan")
	if err := flags.Parse(arguments); err != nil {
		return statequalification.Result{}, err
	}
	if flags.NArg() != 0 {
		return statequalification.Result{}, errors.New("inject-node has unexpected positional arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var parsedAddresses []string
	if addresses != "" {
		parsedAddresses = strings.Split(addresses, ",")
	}
	return renderNode(nodequalification.Run(ctx, nodequalification.Campaign{
		Mode: "inject", Injection: mode, Addresses: parsedAddresses, SecretRoot: secrets, ProbePlan: plan,
	})), nil
}
