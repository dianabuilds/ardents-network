package node

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (observer *nodeObserver) runShortMatrix(ctx context.Context) error {
	if err := observer.stabilizeCurrentAssignment(ctx); err != nil {
		return err
	}
	if err := observer.partialHandshakeFlood(ctx); err != nil {
		return err
	}
	if err := observer.verifySingleSourceFailure(ctx); err != nil {
		return err
	}
	if err := observer.exerciseClockUncertainty(ctx); err != nil {
		return err
	}
	if err := observer.injectPressure(ctx, "node1", "memory"); err != nil {
		return err
	}
	if err := observer.injectPressure(ctx, "node2", "cpu"); err != nil {
		return err
	}
	if err := observer.injectSourcePressure(ctx); err != nil {
		return err
	}
	if err := observer.injectEvidenceFailure(ctx); err != nil {
		return err
	}
	if err := observer.forceEMFILE(ctx); err != nil {
		return err
	}
	if err := observer.forceDiskFull(ctx); err != nil {
		return err
	}
	if err := observer.injectCgroupDrift(ctx); err != nil {
		return err
	}
	return observer.runRestartQuiescence(ctx)
}

func (observer *nodeObserver) verifySingleSourceFailure(ctx context.Context) error {
	observer.setExpectedAbsence(true, "source1")
	defer observer.setExpectedAbsence(false, "source1")
	if _, err := observer.compose(ctx, "kill", "source1"); err != nil {
		return err
	}
	if err := waitNode(ctx, 6*time.Second); err != nil {
		return invalidNodeCampaign(err)
	}
	node1Running, node1Err := observer.running(ctx, "node1")
	node2Running, node2Err := observer.running(ctx, "node2")
	if err := errors.Join(node1Err, node2Err); err != nil {
		return err
	}
	if !node1Running || !node2Running {
		return errors.New("one finite source death removed healthy current Node duties")
	}
	if _, err := observer.compose(ctx, "up", "-d", "source1"); err != nil {
		return err
	}
	return nil
}

func (observer *nodeObserver) exerciseClockUncertainty(ctx context.Context) error {
	observer.setExpectedAbsence(true, "node1")
	defer observer.setExpectedAbsence(false, "node1")
	observer.setClock(1, false)
	if err := observer.waitStopped(ctx, 9*time.Second, "node1"); err != nil {
		return nodeCandidateFailure("clock uncertainty did not withdraw Node1", err)
	}
	observer.captureLogs(ctx, "node1")
	observer.setClock(1, true)
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1"); err != nil {
		return err
	}
	if err := observer.waitServiceReady(ctx, 15*time.Second, "node1"); err != nil {
		return err
	}
	return nil
}

func (observer *nodeObserver) injectCgroupDrift(ctx context.Context) error {
	observer.setExpectedAbsence(true, "node2")
	defer observer.setExpectedAbsence(false, "node2")
	id, err := observer.serviceID(ctx, "node2")
	if err != nil {
		return err
	}
	observer.setFault("cgroup-drift:node2", true)
	if _, err := observer.docker(ctx, "update", "--cpus", "0.5", id); err != nil {
		observer.setFault("cgroup-drift:node2", false)
		return err
	}
	if err := observer.waitStopped(ctx, 5*time.Second, "node2"); err != nil {
		return nodeCandidateFailure("candidate-visible cgroup drift did not withdraw Node2", err)
	}
	observer.captureLogs(ctx, "node2")
	observer.setFault("cgroup-drift:node2", false)
	if _, err = observer.compose(ctx, "up", "-d", "--force-recreate", "node2"); err != nil {
		return err
	}
	if err := observer.waitServiceReady(ctx, 15*time.Second, "node2"); err != nil {
		return err
	}
	return nil
}

func (observer *nodeObserver) injectEvidenceFailure(ctx context.Context) error {
	observer.setExpectedAbsence(true, "node1")
	defer observer.setExpectedAbsence(false, "node1")
	if _, err := observer.compose(ctx, "stop", "--timeout", "5", "node1"); err != nil {
		return err
	}
	observer.captureLogs(ctx, "node1")
	raw, err := observer.compose(ctx, "run", "--rm", "--no-deps", "node1_evidence")
	if appendErr := observer.appendCandidateEvidence(raw); appendErr != nil {
		return appendErr
	}
	if err := nodeMachineCommandError(raw, err, "terminal evidence loss stopped Node admission within two seconds"); err != nil {
		return err
	}
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1"); err != nil {
		return err
	}
	return observer.waitServiceReady(ctx, 15*time.Second, "node1")
}

func (observer *nodeObserver) forceDiskFull(ctx context.Context) error {
	observer.setExpectedAbsence(true, "node1")
	defer observer.setExpectedAbsence(false, "node1")
	if _, err := observer.compose(ctx, "stop", "--timeout", "5", "node1"); err != nil {
		return err
	}
	observer.captureLogs(ctx, "node1")
	raw, runErr := observer.compose(ctx, "run", "--rm", "--no-deps", "node1_diskfull")
	if appendErr := observer.appendCandidateEvidence(append(raw, []byte("\n"+errorText(runErr)+"\n")...)); appendErr != nil {
		return appendErr
	}
	if runErr == nil {
		return errors.New("disk-full persistence cell did not fail closed before Node readiness")
	}
	if !nodeLogContainsExactLine(raw, nodeDiskFullStimulus) {
		return runErr
	}
	if !strings.Contains(runErr.Error(), "no space left on device") {
		return errors.New("disk-full product returned the wrong failure after the ENOSPC stimulus")
	}
	if countNodeLogEvents(raw, "", "READY") != 0 {
		return errors.New("disk-full persistence cell reached Node readiness")
	}
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1"); err != nil {
		return err
	}
	if err := observer.waitServiceReady(ctx, 15*time.Second, "node1"); err != nil {
		return nodeCandidateFailure("node did not recover after the isolated disk-full cell", err)
	}
	return nil
}

func (observer *nodeObserver) injectPressure(ctx context.Context, service, mode string) error {
	logs, err := observer.compose(ctx, "logs", "--no-color", "--no-log-prefix", "--since", "5m", service)
	if err != nil {
		return err
	}
	beforeProtect := countNodeLogEvents(logs, "", "PROTECT")
	beforeNormal := countNodeLogEvents(logs, "resource", "NORMAL")
	id, err := observer.serviceID(ctx, service)
	if err != nil {
		return err
	}
	if _, err := observer.docker(ctx, "exec", "-d", id, "/usr/local/bin/ardents-qualify", "inject-node", "--mode", mode); err != nil {
		return err
	}
	deadline := time.NewTimer(165 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		logs, err = observer.compose(ctx, "logs", "--no-color", "--no-log-prefix", "--since", "5m", service)
		if err != nil {
			return err
		}
		if countNodeLogEvents(logs, "", "PROTECT") > beforeProtect &&
			countNodeLogEvents(logs, "resource", "NORMAL") > beforeNormal {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New(mode + " pressure did not recover through PROTECT to NORMAL after 120 low-watermark seconds")
		case <-ticker.C:
		}
	}
	running, err := observer.running(ctx, service)
	if err != nil {
		return err
	}
	if !running {
		return errors.New(mode + " recoverable pressure terminated the candidate")
	}
	observer.captureLogs(ctx, service)
	return nil
}
