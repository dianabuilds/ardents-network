package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recoverysmoke"
	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func run(arguments []string, output, diagnostics io.Writer) int {
	if code, handled := recoverysmoke.RunOverlapFaultAdapter(arguments, output, diagnostics); handled {
		return code
	}
	if code, handled := recoverysmoke.RunCarrierFaultAdapter(arguments, output, diagnostics); handled {
		return code
	}
	if code, handled := runNodeOfflineCommand(arguments, output, diagnostics); handled {
		return code
	}
	result, err := evaluate(arguments)
	if err != nil {
		result = statequalification.Result{Verdict: "invalid", Reason: err.Error()}
	}
	if encodeErr := json.NewEncoder(output).Encode(result); encodeErr != nil {
		fmt.Fprintln(diagnostics, encodeErr)
		return 2
	}
	switch result.Verdict {
	case "pass":
		return 0
	case "fail":
		return 1
	default:
		return 2
	}
}

func evaluate(arguments []string) (statequalification.Result, error) {
	if len(arguments) == 0 {
		return statequalification.Result{}, errors.New("usage: ardents-qualify (offline|prepare-node|run-node|route-smoke|service-smoke|recovery-smoke|inject-node) [flags]")
	}
	switch arguments[0] {
	case "service-smoke":
		return evaluateDockerSmoke("service", arguments[1:])
	case "recovery-smoke":
		return evaluateDockerSmoke("recovery", arguments[1:])
	case "route-smoke":
		return evaluateRouteSmoke(arguments[1:])
	case "prepare-node":
		return evaluateNodePreparation(arguments[1:])
	case "run-node":
		return evaluateNodeCampaign(arguments[1:])
	case "inject-node":
		return evaluateNodeInjection(arguments[1:])
	case "diskfull-node":
		return evaluateNodeSpecial(arguments[1:], "diskfull-node", "disk-wrapper")
	case "evidence-fault-node":
		return evaluateNodeSpecial(arguments[1:], "evidence-fault-node", "evidence-fault")
	}
	if arguments[0] != "offline" {
		return statequalification.Result{}, errors.New("usage: ardents-qualify (offline|prepare-node|run-node|route-smoke|service-smoke|recovery-smoke|inject-node) [flags]")
	}
	flags := flag.NewFlagSet("offline", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, networkHex, authorityHex, atText, materialsText string
	var threshold int
	flags.StringVar(&root, "state-root", "", "candidate state root")
	flags.StringVar(&networkHex, "network-id", "", "32-byte network identity in hex")
	flags.StringVar(&authorityHex, "authorities", "", "comma-separated Ed25519 public keys in hex")
	flags.IntVar(&threshold, "threshold", 0, "signature threshold")
	flags.StringVar(&atText, "at", "", "verification time in RFC3339")
	flags.StringVar(&materialsText, "materializations", "", "comma-separated materialization files")
	if err := flags.Parse(arguments[1:]); err != nil {
		return statequalification.Result{}, err
	}
	if flags.NArg() != 0 {
		return statequalification.Result{}, errors.New("offline qualification has unexpected positional arguments")
	}
	networkID, authorities, at, materials, err := parseCaseInputs(networkHex, authorityHex, atText, materialsText)
	if err != nil {
		return statequalification.Result{}, err
	}
	return statequalification.Verify(statequalification.Case{
		Root: root, NetworkID: networkID, Authorities: authorities,
		Threshold: threshold, Now: at, Materializations: materials,
	}), nil
}

func parseCaseInputs(networkHex, authorityHex, atText, materialsText string) ([32]byte, map[[32]byte]ed25519.PublicKey, time.Time, [][]byte, error) {
	var networkID [32]byte
	if err := decodeHex(networkHex, networkID[:]); err != nil {
		return networkID, nil, time.Time{}, nil, fmt.Errorf("network-id: %w", err)
	}
	authorities := make(map[[32]byte]ed25519.PublicKey)
	for _, encoded := range strings.Split(authorityHex, ",") {
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeHex(encoded, public); err != nil {
			return networkID, nil, time.Time{}, nil, fmt.Errorf("authority: %w", err)
		}
		authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return networkID, nil, time.Time{}, nil, fmt.Errorf("at: %w", err)
	}
	materials, err := readMaterials(materialsText)
	return networkID, authorities, at, materials, err
}
