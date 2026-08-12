package node

import (
	"context"
	"time"
)

func runDockerCampaign(ctx context.Context, input Campaign) Result {
	observer, err := newNodeObserver(input)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
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
		return observer.result(nodeCampaignVerdict(failure), failure)
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
	return observer.result(nodeCampaignVerdict(err), err)
}
