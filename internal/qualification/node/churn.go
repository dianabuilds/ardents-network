package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func (observer *nodeObserver) runRestartQuiescence(ctx context.Context) error {
	baseline, err := observer.sampleNodeResources(ctx)
	if err != nil {
		return err
	}
	for range 3 {
		observer.setExpectedAbsence(true, "source1", "source2", "node1", "node2")
		restartBarrier := time.Now().UTC()
		if _, err := observer.compose(ctx, "restart", "source1", "source2", "node1", "node2"); err != nil {
			observer.setExpectedAbsence(false, "source1", "source2", "node1", "node2")
			return err
		}
		if err := observer.waitNewReadiness(ctx, restartBarrier); err != nil {
			observer.setExpectedAbsence(false, "source1", "source2", "node1", "node2")
			return err
		}
		observer.setExpectedAbsence(false, "source1", "source2", "node1", "node2")
	}
	if err := waitNode(ctx, 120*time.Second); err != nil {
		return err
	}
	after, err := observer.sampleNodeResources(ctx)
	quiescenceErr := errors.Join(err, validateNodeQuiescentResources(baseline, after))
	if quiescenceErr == nil {
		quiescenceErr = observer.validateNodeProcessQuiescence(ctx)
	}
	if quiescenceErr == nil {
		quiescenceErr = verifyNodeStateQuiescence(observer.input.FixtureRoot)
	}
	receiptErr := byteio.WriteJSON(filepath.Join(observer.input.EvidenceRoot, "quiescence.json"), map[string]any{
		"schema": "ardents-h3-node-quiescence-v1", "baseline": baseline, "after": after,
		"scope":   []string{"cgroup-memory", "process-fds", "process-sockets", "process-pids", "candidate-state"},
		"verdict": nodeQuiescenceVerdict(quiescenceErr), "reason": errorText(quiescenceErr),
	}, 256<<10)
	if receiptErr != nil {
		return errors.Join(quiescenceErr, invalidNodeCampaign(fmt.Errorf("write node quiescence evidence: %w", receiptErr)))
	}
	return quiescenceErr
}

func (observer *nodeObserver) validateNodeProcessQuiescence(ctx context.Context) error {
	ids, err := observer.compose(ctx, "ps", "-q")
	if err != nil {
		return err
	}
	if len(strings.Fields(string(ids))) != 5 {
		return errors.New("node churn did not quiesce to five candidate processes")
	}
	arguments := []string{"stats", "--no-stream", "--format", "{{.PIDs}}"}
	arguments = append(arguments, strings.Fields(string(ids))...)
	counts, err := observer.docker(ctx, arguments...)
	if err != nil {
		return err
	}
	for _, text := range strings.Fields(string(counts)) {
		count, parseErr := strconv.Atoi(text)
		if parseErr != nil || count < 1 || count > 512 {
			return errors.New("node process tree exceeded its PID fuse after churn")
		}
	}
	return nil
}

func validateNodeQuiescentResources(before, after []nodeResourceSnapshot) error {
	baseline := make(map[string]nodeResourceSnapshot, len(before))
	for _, sample := range before {
		baseline[sample.Service] = sample
	}
	current := make(map[string]nodeResourceSnapshot, len(after))
	for _, sample := range after {
		current[sample.Service] = sample
	}
	for _, service := range []string{"source1", "source2", "endpoint", "node1", "node2"} {
		first, beforeOK := baseline[service]
		sample, afterOK := current[service]
		if !beforeOK || !afterOK {
			return invalidNodeCampaign(errors.New("node quiescence sample is missing for " + service))
		}
		if sample.FDs > first.FDs+16 || sample.Sockets > first.Sockets+4 || sample.PIDs > first.PIDs+32 {
			return errors.New("node descriptor, socket, or PID quiescence gate failed for " + service)
		}
		for _, name := range []string{"anon", "sock", "slab"} {
			old, oldOK := nodeRawCounter(first.Raw["memory.stat"], name)
			current, currentOK := nodeRawCounter(sample.Raw["memory.stat"], name)
			absolute := uint64(16 << 20)
			if name == "sock" {
				absolute = 8 << 20
			}
			allowed := max(absolute, old/20)
			if !oldOK || !currentOK {
				return invalidNodeCampaign(errors.New("node memory quiescence counter is missing for " + service + ":" + name))
			}
			if current > old+allowed {
				return errors.New("node memory quiescence gate failed for " + service + ":" + name)
			}
		}
	}
	return nil
}

func nodeQuiescenceVerdict(err error) string {
	if errors.Is(err, errInvalidNodeCampaign) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "invalid"
	}
	if err != nil {
		return "fail"
	}
	return "pass"
}

func (observer *nodeObserver) waitNewReadiness(ctx context.Context, after time.Time) error {
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		var events [2][]byte
		for index, service := range []string{"node1", "node2"} {
			logs, err := observer.compose(ctx, "logs", "--no-color", "--no-log-prefix", "--since",
				after.Format(time.RFC3339Nano), service)
			if err != nil {
				return err
			}
			events[index] = logs
		}
		if nodeSetReadyAfterRestart(events) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("node Node did not regain readiness after churn")
		case <-ticker.C:
		}
	}
}

func nodeSetReadyAfterRestart(events [2][]byte) bool {
	return nodeReadyEvent(events[0]) && nodeReadyEvent(events[1])
}

func verifyNodeStateQuiescence(root string) error {
	entries := 0
	return filepath.WalkDir(filepath.Join(root, "state"), func(path string, entry os.DirEntry, walkErr error) error {
		entries++
		if entries > 512 {
			return errors.New("node state tree exceeds its quiescence bound")
		}
		if walkErr != nil {
			return invalidNodeCampaign(fmt.Errorf("inspect node state quiescence: %w", walkErr))
		}
		if strings.HasPrefix(entry.Name(), ".stage-") || strings.HasPrefix(entry.Name(), ".current-") {
			return errors.New("node state tree retained a temporary resource")
		}
		return nil
	})
}
