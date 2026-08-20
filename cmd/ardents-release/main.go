package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// renderedDecision is the stable offline-import v1 JSON shape.
type renderedDecision struct {
	Schema         string                  `json:"schema"`
	Outcome        releasedecision.Outcome `json:"outcome"`
	Path           string                  `json:"path"`
	Length         int64                   `json:"length"`
	Digest         string                  `json:"digest"`
	Platform       string                  `json:"platform"`
	Architecture   string                  `json:"architecture"`
	Environment    string                  `json:"environment"`
	Network        string                  `json:"network"`
	ReleaseID      string                  `json:"release_identity"`
	ReleaseVersion int64                   `json:"release_version"`
	SourceRev      string                  `json:"source_revision"`
	BuildInputs    string                  `json:"build_input_commitment"`
	BuildID        string                  `json:"build_identity"`
	DependencyID   string                  `json:"dependency_identity"`
	SBOMID         string                  `json:"sbom_identity"`
	Attestation    string                  `json:"attestation_policy"`
	Qualification  string                  `json:"qualification"`
	BuildState     string                  `json:"build_state"`
	ProtocolPhase  string                  `json:"protocol_phase"`
	BuildSafety    releasedecision.Outcome `json:"build_safety"`
	Protocol       releasedecision.Outcome `json:"protocol"`
	RootVersion    int64                   `json:"root_version"`
	Floors         floorOut                `json:"floors"`
	Notice         string                  `json:"notice"`
	CustodyNotice  string                  `json:"custody_notice"`
}

func run(arguments []string, output io.Writer, errorOutput io.Writer) (runErr error) {
	if len(arguments) == 0 {
		return errors.New("usage: ardents-release offline-import [flags]")
	}
	if arguments[0] != "offline-import" {
		return fmt.Errorf("unknown subcommand %q", arguments[0])
	}
	flags := flag.NewFlagSet("offline-import", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	raw := &offlineImportFlags{}
	raw.register(flags)
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("offline-import has unexpected positional arguments")
	}
	inputs, err := raw.buildInputs()
	if err != nil {
		return err
	}
	store, err := releasedecision.OpenFloorStore(raw.stateRoot)
	if err != nil {
		return fmt.Errorf("open state root: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close state root: %w", err))
		}
	}()
	decision := releasedecision.Evaluate(context.Background(), inputs, store)
	rendered, err := json.Marshal(renderedDecision{
		Schema:         "ardents-release-decision-v1",
		Outcome:        decision.Outcome,
		Path:           decision.Path,
		Length:         decision.Length,
		Digest:         hex.EncodeToString(decision.Digest),
		Platform:       decision.Platform,
		Architecture:   decision.Architecture,
		Environment:    decision.Environment,
		Network:        decision.Network,
		ReleaseID:      decision.ReleaseIdentity,
		ReleaseVersion: decision.ReleaseVersion,
		SourceRev:      decision.SourceRevision,
		BuildInputs:    decision.BuildInputCommitment,
		BuildID:        decision.BuildIdentity,
		DependencyID:   decision.DependencyIdentity,
		SBOMID:         decision.SBOMIdentity,
		Attestation:    decision.AttestationPolicy,
		Qualification:  decision.Qualification,
		BuildState:     decision.BuildState,
		ProtocolPhase:  decision.ProtocolPhase,
		BuildSafety:    decision.BuildSafety,
		Protocol:       decision.Protocol,
		RootVersion:    decision.RootVersion,
		Floors:         floorToJSON(decision.Floors),
		Notice:         decision.Notice,
		CustodyNotice:  decision.CustodyNotice,
	})
	if err != nil {
		return err
	}
	rendered = append(rendered, '\n')
	_, err = output.Write(rendered)
	return err
}
