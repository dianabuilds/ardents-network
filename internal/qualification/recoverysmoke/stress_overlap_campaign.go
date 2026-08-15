package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func runOverlapCampaignCell(ctx context.Context, observer dockerObserver, fixture prepared,
	hostScope hostScopeEvidence, hostClock time.Time, manifest []byte) (string, *Result) {
	const direction = "client-to-publisher"
	for retry := 0; retry < 3; retry++ {
		baseline, err := observer.runNoFailureBaseline(ctx, direction)
		if err != nil {
			result := observer.invalid(fmt.Errorf("S4.3 overlap baseline: %w", err))
			return "", &result
		}
		_, receipt, err := runOverlapAttempt(ctx, observer, fixture, direction, hostScope, hostClock, baseline)
		if err != nil {
			result := observer.invalid(fmt.Errorf("S4.3 overlap receipt: %w", err))
			return "", &result
		}
		if receipt.Candidate == "not-run" && receipt.Observation == "invalid" && retry < 2 {
			continue
		}
		if receipt.Observation != "complete" || receipt.Cleanup != "complete" || receipt.Candidate == "not-run" {
			result := observer.invalid(errors.New("S4.3 overlap: " + receipt.Reason))
			return "", &result
		}
		verdict, path, verifyErr := verifyReplacementAttemptReceipt(ctx, observer, json.RawMessage(manifest), receipt)
		if verifyErr != nil || verdict.Verdict != receipt.Candidate {
			if verifyErr == nil {
				verifyErr = errors.New(verdict.Reason)
			}
			result := observer.invalid(fmt.Errorf("verify S4.3 overlap receipt: %w", verifyErr))
			return "", &result
		}
		if receipt.Candidate == "fail" {
			return path, &Result{Verdict: "fail", Reason: "S4.3 overlap: " + receipt.Reason,
				EvidenceRoot: observer.input.EvidenceRoot, SourceCommit: observer.sourceCommit, ImageID: observer.imageID}
		}
		return path, nil
	}
	result := observer.invalid(errors.New("S4.3 overlap infrastructure retry bound was exhausted"))
	return "", &result
}
