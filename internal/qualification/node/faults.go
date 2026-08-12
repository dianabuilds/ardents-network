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
		return invalidNodeCampaignError{err}
	}
	if _, err := observer.compose(ctx, "kill", "source1"); err != nil {
		return err
	}
	if err := waitNode(ctx, 6*time.Second); err != nil || !observer.running(ctx, "node1") || !observer.running(ctx, "node2") {
		return errors.New("one finite source death removed healthy current Node duties")
	}
	if _, err := observer.compose(ctx, "up", "-d", "source1"); err != nil {
		return err
	}
	observer.setClock(1, false)
	if err := observer.waitStopped(ctx, 9*time.Second, "node1"); err != nil {
		return errors.New("clock uncertainty did not withdraw Node1")
	}
	observer.captureLogs(ctx, "node1")
	observer.setClock(1, true)
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1"); err != nil {
		return err
	}
	if err := observer.waitReady(ctx, 15*time.Second); err != nil {
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
	id, err := observer.serviceID(ctx, "node2")
	if err != nil {
		return err
	}
	if _, err := observer.docker(ctx, "update", "--cpus", "0.5", id); err != nil {
		return err
	}
	if err := observer.waitStopped(ctx, 5*time.Second, "node2"); err != nil {
		return errors.New("candidate-visible cgroup drift did not withdraw Node2")
	}
	observer.captureLogs(ctx, "node2")
	if _, err = observer.compose(ctx, "up", "-d", "--force-recreate", "node2"); err != nil {
		return err
	}
	if err := observer.waitReady(ctx, 15*time.Second); err != nil {
		return err
	}
	return observer.runRestartQuiescence(ctx)
}

func (observer *nodeObserver) injectEvidenceFailure(ctx context.Context) error {
	if _, err := observer.compose(ctx, "stop", "--timeout", "5", "node1"); err != nil {
		return err
	}
	observer.captureLogs(ctx, "node1")
	raw, err := observer.compose(ctx, "run", "--rm", "--no-deps", "node1_evidence")
	if appendErr := observer.appendCandidateEvidence(raw); appendErr != nil {
		return appendErr
	}
	if err != nil || !strings.Contains(string(raw), `"verdict":"pass"`) {
		return errors.New("candidate evidence failure did not produce bounded fail-stop proof")
	}
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1"); err != nil {
		return err
	}
	return observer.waitReady(ctx, 15*time.Second)
}

func (observer *nodeObserver) forceDiskFull(ctx context.Context) error {
	if _, err := observer.compose(ctx, "stop", "--timeout", "5", "node1"); err != nil {
		return err
	}
	observer.captureLogs(ctx, "node1")
	raw, runErr := observer.compose(ctx, "run", "--rm", "--no-deps", "node1_diskfull")
	observer.appendCandidateEvidence(append(raw, []byte("\n"+errorText(runErr)+"\n")...))
	if runErr == nil || !strings.Contains(runErr.Error(), "no space left on device") || countBytes(raw, []byte(`"state":"READY"`)) != 0 {
		return errors.New("disk-full persistence cell did not fail closed before Node readiness")
	}
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "node1"); err != nil {
		return err
	}
	if err := observer.waitReady(ctx, 15*time.Second); err != nil {
		return errors.New("node did not recover after the isolated disk-full cell")
	}
	return nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (observer *nodeObserver) injectPressure(ctx context.Context, service, mode string) error {
	logs, err := observer.compose(ctx, "logs", "--no-color", service)
	if err != nil {
		return err
	}
	beforeProtect := countBytes(logs, []byte(`"state":"PROTECT"`))
	beforeNormal := countBytes(logs, []byte(`"kind":"resource","state":"NORMAL"`))
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
		logs, err = observer.compose(ctx, "logs", "--no-color", service)
		if err == nil && countBytes(logs, []byte(`"state":"PROTECT"`)) > beforeProtect &&
			countBytes(logs, []byte(`"kind":"resource","state":"NORMAL"`)) > beforeNormal {
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
	if !observer.running(ctx, service) {
		return errors.New(mode + " recoverable pressure terminated the candidate")
	}
	observer.captureLogs(ctx, service)
	return nil
}

func (observer *nodeObserver) waitStopped(ctx context.Context, timeout time.Duration, services ...string) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		stopped := true
		for _, service := range services {
			stopped = stopped && !observer.running(ctx, service)
		}
		if stopped {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("node service did not stop at its terminal bound")
		case <-ticker.C:
		}
	}
}

func (observer *nodeObserver) partialHandshakeFlood(ctx context.Context) error {
	raw, err := observer.compose(ctx, "run", "--rm", "--no-deps", "harness")
	if err != nil {
		return errors.New("isolated Harness probe matrix failed: " + err.Error() + ": " + string(bytesTrimSpace(raw)))
	}
	if !observer.running(ctx, "node1") || !observer.running(ctx, "node2") {
		return errors.New("partial-handshake flood escaped the Node connection bound")
	}
	return nil
}

func (observer *nodeObserver) running(ctx context.Context, service string) bool {
	raw, err := observer.compose(ctx, "ps", "-q", service)
	return err == nil && len(bytesTrimSpace(raw)) >= 12
}

func waitNode(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
