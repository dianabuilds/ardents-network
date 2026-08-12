package node

import (
	"context"
)

const nodeEvidenceOwner = "ardents-h3-node-evidence-v1\n"

// Campaign identifies one Node qualification operation. Short, churn-2h, and
// unattended-24h require FixtureRoot, EvidenceRoot, and ComposeFile and forbid
// all special-operation fields. Inject mode requires one of the validated
// Injection variants and only that variant's bounded inputs. Evidence-fault
// and disk-wrapper accept no other fields. Run validates every combination
// before I/O; a zero Campaign is invalid.
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
	return runDockerCampaign(ctx, input)
}
