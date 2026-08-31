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
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/publiccontrol"
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
		return errors.New("usage: ardents-control inspect, inspect-bundle, inspect-transitions, inspect-public-control, inspect-alpha-corpus, or accept-alpha-corpus")
	}
	switch arguments[0] {
	case "inspect":
		return inspectStatements(arguments[1:], output)
	case "inspect-bundle":
		return inspectBundle(arguments[1:], output)
	case "inspect-transitions":
		return inspectTransitions(arguments[1:], output)
	case "inspect-public-control":
		return inspectPublicControl(arguments[1:], output)
	case "inspect-alpha-corpus":
		return inspectAlphaCorpus(arguments[1:], output)
	case "accept-alpha-corpus":
		return acceptAlphaCorpus(arguments[1:], output)
	default:
		return errors.New("usage: ardents-control inspect, inspect-bundle, inspect-transitions, inspect-public-control, inspect-alpha-corpus, or accept-alpha-corpus")
	}
}

func inspectPublicControl(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("inspect-public-control", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var evidencePath, atText, expectedPredecessor string
	var auditFloorGeneration uint64
	flags.StringVar(&evidencePath, "evidence", "", "public-control evidence manifest")
	flags.StringVar(&atText, "at", "", "inspection time in RFC3339")
	flags.Uint64Var(&auditFloorGeneration, "audit-floor-generation", 0, "externally retained public-control transition generation floor")
	flags.StringVar(&expectedPredecessor, "expected-predecessor", "", "exact predecessor candidate digest from external audit evidence")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("public-control inspection arguments are invalid")
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return errors.New("public-control inspection time is invalid")
	}
	raw, err := readControlFile(evidencePath, publiccontrol.MaximumEvidenceManifestSize)
	if err != nil {
		return err
	}
	report, err := publiccontrol.InspectAt(raw, publiccontrol.InspectionConfig{At: at.UTC(), AuditFloorGeneration: auditFloorGeneration, ExpectedPredecessor: expectedPredecessor})
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(report)
}

func inspectAlphaCorpus(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("inspect-alpha-corpus", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var catalogPath, corpusPath, disclosureKey, corpusKey, networkText, atText string
	flags.StringVar(&catalogPath, "catalog", "", "ACA2 catalog file")
	flags.StringVar(&corpusPath, "corpus", "", "signed alpha corpus file")
	flags.StringVar(&disclosureKey, "disclosure-key", "", "ACA2 disclosure public key in lowercase hex")
	flags.StringVar(&corpusKey, "corpus-key", "", "alpha corpus authority public key in lowercase hex")
	flags.StringVar(&networkText, "network", "", "Ardents network ID in lowercase hex")
	flags.StringVar(&atText, "at", "", "decision time in RFC3339")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("alpha corpus inspection arguments are invalid")
	}
	disclosure, err := decodePublicKey(disclosureKey)
	if err != nil {
		return err
	}
	corpusAuthority, err := decodePublicKey(corpusKey)
	if err != nil {
		return errors.New("alpha corpus authority key is invalid")
	}
	network, err := decodeIdentifier(networkText)
	if err != nil {
		return errors.New("alpha corpus network is invalid")
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return errors.New("alpha corpus inspection time is invalid")
	}
	catalog, err := readControlFile(catalogPath, alphacontrol.MaximumCatalogSize)
	if err != nil {
		return err
	}
	corpusRaw, err := readControlFile(corpusPath, 4096)
	if err != nil {
		return err
	}
	corpus, outcome := inspection.VerifyACA2Corpus(catalog, disclosure, corpusAuthority, corpusRaw, network, at.UTC())
	if outcome != alphacontrol.OutcomeAccepted || corpus == nil {
		return errors.New("alpha corpus control was not accepted")
	}
	return json.NewEncoder(output).Encode(struct {
		Schema  string `json:"schema"`
		Cohort  string `json:"cohort"`
		Corpus  string `json:"corpus"`
		Network string `json:"network"`
		Serial  uint64 `json:"serial"`
	}{Schema: "ardents-alpha-corpus-report-v1", Cohort: corpus.Cohort(), Corpus: string(alphacontrol.OutcomeAccepted), Network: networkText, Serial: corpus.Serial()})
}

// inspectStatements is a low-level statement-integrity diagnostic. The
// alpha-control participant flow is inspect-bundle, which first verifies the
// Alpha Enrollment Pin and then invokes the Release and Network State verifiers.
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
