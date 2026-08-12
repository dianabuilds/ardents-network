package node

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (observer *nodeObserver) runSustainedCampaign(ctx context.Context) error {
	if err := observer.stabilizeCurrentAssignment(ctx); err != nil {
		return err
	}
	duration := campaignDuration(observer.input.Mode)
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	health := time.NewTicker(30 * time.Second)
	defer health.Stop()
	probe := time.NewTicker(15 * time.Minute)
	defer probe.Stop()
	var churn *time.Ticker
	var churnChannel <-chan time.Time
	if observer.input.Mode == "churn-2h" {
		churn = time.NewTicker(5 * time.Minute)
		defer churn.Stop()
		churnChannel = churn.C
	}
	for {
		select {
		case <-ctx.Done():
			return invalidNodeCampaignError{ctx.Err()}
		case <-deadline.C:
			if observer.input.Mode == "churn-2h" {
				return observer.runRestartQuiescence(ctx)
			}
			return observer.ensureCandidateSet(ctx)
		case <-health.C:
			if err := observer.ensureCandidateSet(ctx); err != nil {
				return err
			}
		case <-probe.C:
			if err := observer.partialHandshakeFlood(ctx); err != nil {
				return invalidNodeCampaignError{err}
			}
		case <-churnChannel:
			if err := observer.restartCandidateSet(ctx); err != nil {
				return err
			}
		}
	}
}

func (observer *nodeObserver) stabilizeCurrentAssignment(ctx context.Context) error {
	if err := observer.waitStopped(ctx, 12*time.Second, "node1", "node2"); err != nil {
		return errors.New("automatic reassignment did not drain both original duties")
	}
	observer.captureLogs(ctx, "node1", "node2")
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1", "node2"); err != nil {
		return invalidNodeCampaignError{err}
	}
	return observer.waitReady(ctx, 15*time.Second)
}

func (observer *nodeObserver) restartCandidateSet(ctx context.Context) error {
	before := [2]int{}
	for index, service := range []string{"node1", "node2"} {
		logs, err := observer.compose(ctx, "logs", "--no-color", service)
		if err != nil {
			return invalidNodeCampaignError{err}
		}
		before[index] = countBytes(logs, []byte(`"state":"READY"`))
	}
	if _, err := observer.compose(ctx, "restart", "source1", "source2", "node1", "node2"); err != nil {
		return invalidNodeCampaignError{err}
	}
	return observer.waitNewReadiness(ctx, before)
}

func (observer *nodeObserver) ensureCandidateSet(ctx context.Context) error {
	processes, err := observer.composeBounded(ctx, 32<<10, "ps", "--status", "running", "--format", "{{.Service}}")
	if err != nil {
		return invalidNodeCampaignError{err}
	}
	for _, service := range []string{"source1", "source2", "endpoint", "node1", "node2"} {
		if !containsNodeLine(string(processes), service) {
			return errors.New("node sustained candidate exited: " + service)
		}
	}
	return nil
}

func containsNodeLine(raw, target string) bool {
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

type invalidNodeCampaignError struct{ error }
