package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// acceptAlphaCorpus verifies one explicitly supplied ACA2/corpus pair against
// a v3 enrolled bundle, then advances only the explicitly named local corpus
// floor. It neither fetches bytes nor starts an Endpoint.
func acceptAlphaCorpus(arguments []string, output io.Writer) (resultErr error) {
	flags := flag.NewFlagSet("accept-alpha-corpus", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var enrollmentPath, artifact, controlRoot, corpusRoot, catalogPath, corpusPath, atText string
	flags.StringVar(&enrollmentPath, "enrollment", "", "alpha enrollment input JSON")
	flags.StringVar(&artifact, "artifact", "", "exact enrolled artifact path")
	flags.StringVar(&controlRoot, "control-state-root", "", "reader-owned ACA1 inspection state root")
	flags.StringVar(&corpusRoot, "corpus-state-root", "", "Endpoint-owned alpha corpus state root")
	flags.StringVar(&catalogPath, "catalog", "", "ACA2 catalog file")
	flags.StringVar(&corpusPath, "corpus", "", "signed alpha corpus file")
	flags.StringVar(&atText, "at", "", "decision time in RFC3339")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || controlRoot == "" || corpusRoot == "" {
		return errors.New("alpha corpus acceptance arguments are invalid")
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return errors.New("alpha corpus acceptance time is invalid")
	}
	input, err := enrollment.ReadClosedAlphaInput(enrollmentPath)
	if err != nil {
		return err
	}
	request := input.Request(artifact, at)
	if err := verifyEnrolledControlCommand(request); err != nil {
		return err
	}
	catalog, err := readControlFile(catalogPath, alphacontrol.MaximumCatalogSize)
	if err != nil {
		return err
	}
	corpusRaw, err := readControlFile(corpusPath, 4096)
	if err != nil {
		return err
	}
	report, err := inspection.InspectCorpus(context.Background(), inspection.CorpusConfig{Control: inspection.Config{Root: controlRoot,
		Enrollment: request, At: at.UTC()}, Catalog: catalog, Corpus: corpusRaw})
	if err != nil {
		return err
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: corpusRoot, Authority: report.CorpusAuthority,
		Cohort: report.Corpus.Cohort(), Network: report.Control.NetworkID})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, floor.Close()) }()
	if err := floor.Observe(report.Corpus); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema  string `json:"schema"`
		Cohort  string `json:"cohort"`
		Corpus  string `json:"corpus"`
		Network string `json:"network"`
		Serial  uint64 `json:"serial"`
	}{Schema: "ardents-alpha-corpus-acceptance-v1", Cohort: report.Corpus.Cohort(), Corpus: string(alphacontrol.OutcomeAccepted),
		Network: hex.EncodeToString(report.Control.NetworkID[:]), Serial: report.Corpus.Serial()})
}

// verifyEnrolledControlCommand makes the command's executable provenance part
// of the accepting operation. The Endpoint artifact is verified independently
// by the same enrollment request; the v3 control command is a separate,
// manifest-pinned participant tool and never Release metadata.
func verifyEnrolledControlCommand(request enrollment.Request) error {
	verified, err := enrollment.Verify(request)
	if err != nil {
		return err
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	expectedName := enrollment.ExecutableArtifactName("ardents-control", platform)
	if request.Pin.Platform != platform || verified.ControlArtifactName != expectedName || len(verified.ControlArtifact) == 0 {
		return errors.New("alpha corpus acceptance requires an enrolled v3 control command")
	}
	return enrollment.VerifyRunningCompanion(request, expectedName, verified.ControlArtifact)
}
