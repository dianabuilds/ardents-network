package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
)

var componentNames = [...]string{"release.ac1", "network.ac1", "compatibility.ac1"}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return errors.New("usage: ardents-control inspect or inspect-bundle")
	}
	switch arguments[0] {
	case "inspect":
		return inspectStatements(arguments[1:], output)
	case "inspect-bundle":
		return inspectBundle(arguments[1:], output)
	default:
		return errors.New("usage: ardents-control inspect or inspect-bundle")
	}
}

// inspectStatements is a low-level statement-integrity diagnostic. The H4-6A
// participant flow is inspect-bundle, which first verifies the Alpha
// Enrollment Pin and then invokes the Release and Network State verifiers.
func inspectStatements(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var directory, stateRoot, encodedKey, releaseKey, networkKey, compatibilityKey, atText string
	flags.StringVar(&directory, "directory", "", "fixed catalog directory")
	flags.StringVar(&stateRoot, "state-root", "", "reader-owned floor root")
	flags.StringVar(&encodedKey, "disclosure-key", "", "Ed25519 public key in lowercase hex")
	flags.StringVar(&releaseKey, "release-key", "", "Ed25519 release component public key in lowercase hex")
	flags.StringVar(&networkKey, "network-key", "", "Ed25519 network component public key in lowercase hex")
	flags.StringVar(&compatibilityKey, "compatibility-key", "", "Ed25519 compatibility component public key in lowercase hex")
	flags.StringVar(&atText, "at", "", "decision time in RFC3339")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("alpha control inspect arguments are invalid")
	}
	public, err := decodePublicKey(encodedKey)
	if err != nil {
		return err
	}
	componentKeys := [3]string{releaseKey, networkKey, compatibilityKey}
	var roots [3]ed25519.PublicKey
	for index, encoded := range componentKeys {
		roots[index], err = decodePublicKey(encoded)
		if err != nil {
			return errors.New("alpha control component root is invalid")
		}
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return errors.New("alpha control inspection time is invalid")
	}
	catalog, components, err := readDirectory(directory)
	if err != nil {
		return err
	}
	reader, err := alphacontrol.OpenReader(alphacontrol.ReaderConfig{Root: stateRoot, DisclosureKey: public, ComponentKeys: roots, Clock: func() time.Time { return at.UTC() }})
	if err != nil {
		return err
	}
	defer reader.Close()
	result, err := reader.Inspect(catalog, components, func(alphacontrol.Component, alphacontrol.ComponentStatement, time.Time) alphacontrol.Outcome {
		return alphacontrol.OutcomeAccepted
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema     string                              `json:"schema"`
		Catalog    alphacontrol.Outcome                `json:"catalog"`
		Components [3]alphacontrol.ComponentInspection `json:"components"`
	}{Schema: "ardents-alpha-control-report-v1", Catalog: result.Catalog, Components: result.Components})
}

func inspectBundle(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("inspect-bundle", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var enrollmentPath, artifact, stateRoot, atText string
	flags.StringVar(&enrollmentPath, "enrollment", "", "alpha enrollment input JSON")
	flags.StringVar(&artifact, "artifact", "", "exact enrolled artifact path")
	flags.StringVar(&stateRoot, "state-root", "", "reader-owned inspection state root")
	flags.StringVar(&atText, "at", "", "decision time in RFC3339")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("alpha control bundle inspection arguments are invalid")
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return errors.New("alpha control inspection time is invalid")
	}
	input, err := readEnrollmentInput(enrollmentPath)
	if err != nil {
		return err
	}
	report, err := inspection.Inspect(context.Background(), inspection.Config{Root: stateRoot, Enrollment: enrollment.Request{BundleRoot: input.BundleRoot, ExecutablePath: artifact,
		Pin:         enrollment.Pin{Cohort: input.Cohort, Release: input.Release, Platform: input.Platform, ManifestSHA256: input.ManifestSHA256},
		Environment: input.Environment, Network: input.Network, TargetPath: input.TargetPath, Architecture: runtime.GOARCH, ReferenceTime: at.UTC()}, At: at.UTC()})
	encoded := json.NewEncoder(output).Encode(struct {
		Schema        string                              `json:"schema"`
		Catalog       alphacontrol.Outcome                `json:"catalog"`
		Components    [3]alphacontrol.ComponentInspection `json:"components"`
		Release       string                              `json:"release"`
		NetworkEpoch  uint64                              `json:"network_epoch"`
		NetworkDigest string                              `json:"network_digest"`
	}{Schema: "ardents-alpha-control-report-v1", Catalog: report.Inspection.Catalog, Components: report.Inspection.Components,
		Release: report.Release, NetworkEpoch: report.NetworkEpoch, NetworkDigest: hex.EncodeToString(report.NetworkDigest[:])})
	return errors.Join(err, encoded)
}

type alphaEnrollmentInput struct {
	Schema         string `json:"schema"`
	BundleRoot     string `json:"bundle_root"`
	Cohort         string `json:"cohort"`
	Release        string `json:"release"`
	Platform       string `json:"platform"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Environment    string `json:"environment"`
	Network        string `json:"network"`
	TargetPath     string `json:"target_path"`
}

func readEnrollmentInput(path string) (alphaEnrollmentInput, error) {
	if path == "" {
		return alphaEnrollmentInput{}, errors.New("alpha enrollment input is required")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 || info.Size() > 16<<10 {
		return alphaEnrollmentInput{}, errors.New("alpha enrollment input is not a bounded regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return alphaEnrollmentInput{}, err
	}
	var input alphaEnrollmentInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Schema != "ardents-alpha-enrollment-input-v1" {
		return alphaEnrollmentInput{}, errors.New("alpha enrollment input is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return alphaEnrollmentInput{}, errors.New("alpha enrollment input is invalid")
	}
	return input, nil
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PublicKeySize || hex.EncodeToString(raw) != encoded {
		return nil, errors.New("alpha control disclosure key is invalid")
	}
	return ed25519.PublicKey(raw), nil
}

func readDirectory(path string) ([]byte, [3][]byte, error) {
	var components [3][]byte
	directory, err := filepath.Abs(path)
	if err != nil {
		return nil, components, err
	}
	if _, err := os.ReadDir(directory); err != nil {
		return nil, components, errors.New("alpha control directory is incomplete")
	}
	read := func(name string) ([]byte, error) {
		info, statErr := os.Lstat(filepath.Join(directory, name))
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 || info.Size() > 64<<20 {
			return nil, errors.New("alpha control component is not a bounded regular file")
		}
		return os.ReadFile(filepath.Join(directory, name))
	}
	catalog, err := read("catalog.ac1")
	if err != nil {
		return nil, components, err
	}
	for index, name := range componentNames {
		components[index], err = read(name)
		if err != nil {
			return nil, components, err
		}
	}
	return catalog, components, nil
}
