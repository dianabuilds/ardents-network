package node

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/node/fixture"
)

const nodeEvidenceOwner = "ardents-h3-node-evidence-v1\n"

// Campaign identifies one externally observed Docker campaign.
type Campaign struct {
	FixtureRoot  string
	EvidenceRoot string
	ComposeFile  string
	Mode         string
	Addresses    []string
	SecretRoot   string
	Injection    string
	ProbePlan    string
}

// Run owns the Docker candidates, external clock, faults, evidence, and cleanup.
func Run(ctx context.Context, input Campaign) Result {
	if input.Mode == "evidence-fault" {
		if err := validateNodeSpecialInput(input, "evidence-fault"); err != nil {
			return Result{Verdict: "invalid", Reason: err.Error()}
		}
		return runNodeEvidenceFault(ctx)
	}
	if input.Mode == "disk-wrapper" {
		if err := validateNodeSpecialInput(input, "disk-wrapper"); err != nil {
			return Result{Verdict: "invalid", Reason: err.Error()}
		}
		return runNodeDiskWrapper()
	}
	if input.Mode == "inject" {
		if err := validateNodeInjectionInput(input); err != nil {
			return Result{Verdict: "invalid", Reason: err.Error()}
		}
		return runNodeInjection(ctx, input)
	}
	input, err := validateCampaign(input)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	observer, err := newNodeObserver(input)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	defer observer.close()
	if _, err := observer.compose(ctx, "config", "--quiet"); err != nil {
		return observer.result("invalid", err)
	}
	if err := observer.capturePreflightEvidence(ctx); err != nil {
		return observer.result("invalid", err)
	}
	observer.start()
	if _, err := observer.compose(ctx, "up", "--build", "-d", "--force-recreate"); err != nil {
		return observer.result("invalid", err)
	}
	defer observer.cleanup()
	if err := observer.captureCandidateIdentity(ctx); err != nil {
		return observer.result("invalid", err)
	}
	if err := observer.startCollector(ctx); err != nil {
		return observer.result("invalid", err)
	}
	if err := observer.waitReady(ctx, 30*time.Second); err != nil {
		failure := nodeCandidateFailure("node candidates did not reach READY", err)
		if errors.Is(failure, errInvalidNodeCampaign) || errors.Is(failure, context.Canceled) || errors.Is(failure, context.DeadlineExceeded) {
			return observer.result("invalid", failure)
		}
		return observer.result("fail", failure)
	}
	if err := observer.captureInitialResources(ctx); err != nil {
		return observer.result("invalid", err)
	}
	observer.startSamples()
	mode, _ := selectNodeCampaignMode(input.Mode)
	if mode.short {
		err = observer.runShortMatrix(ctx)
	} else {
		err = observer.runSustainedCampaign(ctx)
	}
	if err != nil {
		if errors.Is(err, errInvalidNodeCampaign) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return observer.result("invalid", err)
		}
		return observer.result("fail", err)
	}
	return observer.result("pass", nil)
}

func validateCampaign(input Campaign) (Campaign, error) {
	if campaignDuration(input.Mode) < 0 {
		return input, errors.New("node campaign mode is invalid")
	}
	if err := validateNodeSpecialInput(input, input.Mode); err != nil {
		return input, err
	}
	var err error
	input.FixtureRoot, err = filepath.Abs(input.FixtureRoot)
	if err != nil {
		return input, err
	}
	input.EvidenceRoot, err = filepath.Abs(input.EvidenceRoot)
	if err != nil {
		return input, err
	}
	input.ComposeFile, err = filepath.Abs(input.ComposeFile)
	if err != nil {
		return input, err
	}
	if err := fixture.Validate(input.FixtureRoot); err != nil {
		return input, err
	}
	if _, err := os.Stat(input.ComposeFile); err != nil {
		return input, err
	}
	if err := os.Mkdir(input.EvidenceRoot, 0o700); err != nil {
		return input, err
	}
	if err := os.WriteFile(filepath.Join(input.EvidenceRoot, ".ardents-node-evidence"), []byte(nodeEvidenceOwner), 0o600); err != nil {
		return input, err
	}
	raw, marshalErr := json.Marshal(input)
	if marshalErr != nil || len(raw) > 16<<10 {
		return input, errors.Join(marshalErr, errors.New("node campaign input exceeds its bound"))
	}
	if err := os.WriteFile(filepath.Join(input.EvidenceRoot, "campaign-input.json"), append(raw, '\n'), 0o600); err != nil {
		return input, err
	}
	return input, nil
}
