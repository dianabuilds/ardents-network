package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"time"

	nodequalification "github.com/dianabuilds/ardents-network/internal/qualification/node"
	"github.com/dianabuilds/ardents-network/internal/qualification/node/fixture"
	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func evaluateNodePreparation(arguments []string) (statequalification.Result, error) {
	flags := flag.NewFlagSet("prepare-node", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, atText, ardentsPath string
	var linuxUIDOwnership bool
	flags.StringVar(&root, "root", "", "new out-of-repository fixture root")
	flags.StringVar(&atText, "at", "", "campaign time in RFC3339")
	flags.StringVar(&ardentsPath, "ardents", "", "optional absolute ardents binary path")
	flags.BoolVar(&linuxUIDOwnership, "linux-uid-ownership", false, "assign fixed Docker role UIDs on a Linux host")
	if err := flags.Parse(arguments); err != nil {
		return statequalification.Result{}, err
	}
	if flags.NArg() != 0 {
		return statequalification.Result{}, errors.New("prepare-node has unexpected positional arguments")
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return statequalification.Result{}, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return statequalification.Result{}, err
	}
	if ardentsPath != "" {
		ardentsPath, err = filepath.Abs(ardentsPath)
		if err != nil {
			return statequalification.Result{}, err
		}
	}
	if err := fixture.Prepare(fixture.PrepareConfig{Root: root, Now: at, ArdentsPath: ardentsPath,
		LinuxUIDOwnership: linuxUIDOwnership}); err != nil {
		return statequalification.Result{}, err
	}
	return statequalification.Result{Verdict: "pass", Reason: "node fixture prepared"}, nil
}

func evaluateNodeSpecial(arguments []string, command, mode string) (statequalification.Result, error) {
	if len(arguments) != 0 {
		return statequalification.Result{}, errors.New(command + " has unexpected arguments")
	}
	return renderNode(nodequalification.Run(context.Background(), nodequalification.Campaign{Mode: mode})), nil
}

func renderNode(result nodequalification.Result) statequalification.Result {
	return statequalification.Result{
		Verdict: result.Verdict, Reason: result.Reason,
		EvidenceRoot: result.EvidenceRoot, EvidenceDigest: result.EvidenceDigest,
	}
}
