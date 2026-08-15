package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func runImpairedCampaignCell(ctx context.Context, observer dockerObserver, fixture prepared, direction string,
	hostScope hostScopeEvidence, hostClock time.Time, manifest []byte) (string, *Result) {
	for retry := 0; retry < 3; retry++ {
		receipt, err := runImpairedAttempt(ctx, observer, fixture, direction, hostScope, hostClock)
		if err != nil {
			result := observer.invalid(fmt.Errorf("S4.3 %s impaired receipt: %w", direction, err))
			return "", &result
		}
		if receipt.Candidate == "not-run" && receipt.Observation == "invalid" && retry < 2 {
			continue
		}
		if receipt.Observation != "complete" || receipt.Cleanup != "complete" || receipt.Candidate == "not-run" {
			result := observer.invalid(errors.New("S4.3 " + direction + " impaired: " + receipt.Reason))
			return "", &result
		}
		verdict, path, verifyErr := verifyReplacementAttemptReceipt(ctx, observer, json.RawMessage(manifest), receipt)
		if verifyErr != nil || verdict.Verdict != receipt.Candidate {
			if verifyErr == nil {
				verifyErr = errors.New(verdict.Reason)
			}
			result := observer.invalid(fmt.Errorf("verify S4.3 %s impaired receipt: %w", direction, verifyErr))
			return "", &result
		}
		if receipt.Candidate == "fail" {
			return path, &Result{Verdict: "fail", Reason: "S4.3 " + direction + " impaired: " + receipt.Reason,
				EvidenceRoot: observer.input.EvidenceRoot, SourceCommit: observer.sourceCommit, ImageID: observer.imageID}
		}
		return path, nil
	}
	result := observer.invalid(errors.New("S4.3 impaired infrastructure retry bound was exhausted"))
	return "", &result
}
