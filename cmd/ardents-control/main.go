package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
)

const commandUsage = "usage: ardents-control inspect-bundle, inspect-transitions, prepare-closed-profile, sign-closed-profile, inspect-closed-profile, inspect-closed-issuer-profile, prepare-qualification-evidence, prepare-qualification-catalog, prepare-qualification-node-record, or prepare-qualification-epoch"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return errors.New(commandUsage)
	}
	switch arguments[0] {
	case "inspect-bundle":
		return inspectBundle(arguments[1:], output)
	case "inspect-transitions":
		return inspectTransitions(arguments[1:], output)
	case "prepare-qualification-evidence":
		return prepareEvidence(arguments[1:], output)
	case "prepare-qualification-catalog":
		return prepareCatalog(arguments[1:], output)
	case "prepare-qualification-node-record", "prepare-qualification-epoch":
		return prepareQualificationNetwork(arguments[0], arguments[1:], output)
	case "inspect-alpha-corpus":
		return errors.New("inspect-alpha-corpus is retired")
	case "accept-alpha-corpus":
		return errors.New("accept-alpha-corpus is retired")
	case "prepare-closed-profile":
		return prepareClosedProfile(arguments[1:], output)
	case "sign-closed-profile":
		return signClosedProfile(arguments[1:], output)
	case "inspect-closed-issuer-profile":
		return inspectClosedIssuerProfile(arguments[1:], output)
	case "inspect-closed-profile":
		return inspectClosedProfile(arguments[1:], output)
	default:
		return errors.New(commandUsage)
	}
}

func inspectBundle(arguments []string, output io.Writer) error {
	report, inspectionErr := inspectBundleReport("inspect-bundle", arguments)
	return errors.Join(inspectionErr, writeBundleInspectionReport(output, report))
}

func inspectTransitions(arguments []string, output io.Writer) error {
	report, inspectionErr := inspectBundleReport("inspect-transitions", arguments)
	return errors.Join(inspectionErr, json.NewEncoder(output).Encode(transitionInspectionReport(report)))
}

func inspectBundleReport(command string, arguments []string) (inspection.Report, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var enrollmentPath, artifact, stateRoot, atText string
	flags.StringVar(&enrollmentPath, "enrollment", "", "alpha enrollment input JSON")
	flags.StringVar(&artifact, "artifact", "", "exact enrolled artifact path")
	flags.StringVar(&stateRoot, "state-root", "", "reader-owned inspection state root")
	flags.StringVar(&atText, "at", "", "decision time in RFC3339")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return inspection.Report{}, fmt.Errorf("alpha control %s arguments are invalid", command)
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return inspection.Report{}, fmt.Errorf("alpha control %s time is invalid", command)
	}
	input, err := enrollment.ReadClosedAlphaInput(enrollmentPath)
	if err != nil {
		return inspection.Report{}, err
	}
	return inspection.Inspect(context.Background(), inspection.Config{Root: stateRoot, Enrollment: input.Request(artifact, at), At: at.UTC()})
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PublicKeySize || hex.EncodeToString(raw) != encoded {
		return nil, errors.New("alpha control disclosure key is invalid")
	}
	return ed25519.PublicKey(raw), nil
}

func decodeIdentifier(encoded string) ([32]byte, error) {
	var result [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(result) || hex.EncodeToString(raw) != encoded {
		return result, errors.New("alpha control identifier is invalid")
	}
	copy(result[:], raw)
	return result, nil
}

func readControlFile(path string, maximum int64) ([]byte, error) {
	if path == "" {
		return nil, errors.New("control file is required")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 || info.Size() > maximum {
		return nil, errors.New("control file is not a bounded regular file")
	}
	return os.ReadFile(path)
}
