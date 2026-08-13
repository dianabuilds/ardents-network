package node

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (observer *nodeObserver) setExpectedAbsence(active bool, services ...string) {
	observer.mu.Lock()
	for _, service := range services {
		observer.activeFaults["absence:"+service] = active
	}
	faults := copyNodeFaults(observer.activeFaults)
	observer.mu.Unlock()
	observer.notifyResourceReset(faults)
}

func (observer *nodeObserver) waitStopped(ctx context.Context, timeout time.Duration, services ...string) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		stopped := true
		for _, service := range services {
			running, err := observer.running(ctx, service)
			if err != nil {
				return err
			}
			stopped = stopped && !running
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
	raw, runErr := observer.compose(ctx, "run", "--rm", "--no-deps", "harness")
	if err := observer.appendCandidateEvidence(raw); err != nil {
		return err
	}
	if err := nodeMachineCommandError(raw, runErr, "node probe faults remained bounded"); err != nil {
		return fmt.Errorf("isolated Harness probe matrix: %w", err)
	}
	node1Running, node1Err := observer.running(ctx, "node1")
	node2Running, node2Err := observer.running(ctx, "node2")
	if err := errors.Join(node1Err, node2Err); err != nil {
		return err
	}
	if !node1Running || !node2Running {
		return errors.New("partial-handshake flood escaped the Node connection bound")
	}
	return nil
}

func (observer *nodeObserver) running(ctx context.Context, service string) (bool, error) {
	raw, err := observer.compose(ctx, "ps", "-q", service)
	if err != nil {
		return false, err
	}
	identity := bytesTrimSpace(raw)
	if len(identity) == 0 {
		return false, nil
	}
	if len(identity) < 12 || len(identity) > 64 {
		return false, invalidNodeCampaign(errors.New("node service status identity is invalid"))
	}
	return true, nil
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
