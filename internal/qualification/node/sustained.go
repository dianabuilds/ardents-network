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
	mode, _ := selectNodeCampaignMode(observer.input.Mode)
	campaign, cancel := context.WithTimeout(ctx, mode.duration)
	defer cancel()
	health := time.NewTicker(30 * time.Second)
	defer health.Stop()
	probe := time.NewTicker(15 * time.Minute)
	defer probe.Stop()
	var churn *time.Ticker
	var churnChannel <-chan time.Time
	churnCycle := 0
	if mode.churn {
		churn = time.NewTicker(5 * time.Minute)
		defer churn.Stop()
		churnChannel = churn.C
	}
	for {
		select {
		case <-campaign.Done():
			if ctx.Err() != nil {
				return invalidNodeCampaign(ctx.Err())
			}
			if mode.churn {
				return observer.runRestartQuiescence(ctx)
			}
			return observer.ensureCandidateSet(ctx)
		case <-health.C:
			if err := observer.ensureCandidateSet(ctx); err != nil {
				return err
			}
		case <-probe.C:
			if err := observer.partialHandshakeFlood(ctx); err != nil {
				return err
			}
		case <-churnChannel:
			churnCycle++
			if err := observer.runChurnResourceCell(campaign, churnCycle); err != nil {
				return err
			}
			if err := observer.restartCandidateSet(ctx); err != nil {
				return err
			}
		}
	}
}

func (observer *nodeObserver) runChurnResourceCell(ctx context.Context, cycle int) error {
	service, pressure := churnResourceCell(cycle)
	if service == "source" {
		return observer.injectSourcePressure(ctx)
	}
	return observer.injectPressure(ctx, service, pressure)
}

func churnResourceCell(cycle int) (string, string) {
	if cycle%6 == 0 {
		return "source", "cpu"
	}
	if cycle%2 == 0 {
		return "node2", "cpu"
	}
	return "node1", "memory"
}

func (observer *nodeObserver) stabilizeCurrentAssignment(ctx context.Context) error {
	observer.setExpectedAbsence(true, "node1", "node2")
	defer observer.setExpectedAbsence(false, "node1", "node2")
	if err := observer.waitStopped(ctx, 12*time.Second, "node1", "node2"); err != nil {
		return nodeCandidateFailure("automatic reassignment did not drain both original duties", err)
	}
	observer.captureLogs(ctx, "node1", "node2")
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1", "node2"); err != nil {
		return invalidNodeCampaign(err)
	}
	return observer.waitReady(ctx, 15*time.Second)
}

func (observer *nodeObserver) restartCandidateSet(ctx context.Context) error {
	observer.setExpectedAbsence(true, "source1", "source2", "node1", "node2")
	defer observer.setExpectedAbsence(false, "source1", "source2", "node1", "node2")
	restartBarrier := time.Now().UTC()
	if _, err := observer.compose(ctx, "restart", "source1", "source2", "node1", "node2"); err != nil {
		return invalidNodeCampaign(err)
	}
	return observer.waitNewReadiness(ctx, restartBarrier)
}

func (observer *nodeObserver) ensureCandidateSet(ctx context.Context) error {
	processes, err := observer.composeBounded(ctx, 32<<10, "ps", "--status", "running", "--format", "{{.Service}}")
	if err != nil {
		return invalidNodeCampaign(err)
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
