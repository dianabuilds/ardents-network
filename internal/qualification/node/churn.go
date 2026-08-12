package node

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (observer *nodeObserver) runRestartQuiescence(ctx context.Context) error {
	baseline, err := observer.sampleNodeResources(ctx, time.Now())
	if err != nil {
		return err
	}
	for range 3 {
		before := [2]int{}
		for index, service := range []string{"node1", "node2"} {
			logs, _ := observer.compose(ctx, "logs", "--no-color", service)
			before[index] = countBytes(logs, []byte(`"state":"READY"`))
		}
		if _, err := observer.compose(ctx, "restart", "source1", "source2", "node1", "node2"); err != nil {
			return err
		}
		if err := observer.waitNewReadiness(ctx, before); err != nil {
			return err
		}
	}
	if err := waitNode(ctx, 120*time.Second); err != nil {
		return err
	}
	after, err := observer.sampleNodeResources(ctx, time.Now())
	if err != nil || !nodeQuiescentResources(baseline, after) {
		return errors.Join(err, errors.New("node candidate resources did not quiesce after 120 seconds"))
	}
	ids, err := observer.compose(ctx, "ps", "-q")
	if err != nil || len(strings.Fields(string(ids))) != 5 {
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
	return verifyNodeStateQuiescence(observer.input.FixtureRoot)
}

func nodeQuiescentResources(before, after []nodeResourceSnapshot) bool {
	baseline := make(map[string]nodeResourceSnapshot, len(before))
	for _, sample := range before {
		baseline[sample.Service] = sample
	}
	checked := 0
	for _, sample := range after {
		first, found := baseline[sample.Service]
		if !found || sample.Service == "endpoint" {
			continue
		}
		if sample.FDs > first.FDs+16 || sample.Sockets > first.Sockets+4 || sample.PIDs > first.PIDs+32 {
			return false
		}
		for _, name := range []string{"anon", "sock", "slab"} {
			old, oldOK := nodeRawCounter(first.Raw["memory.stat"], name)
			current, currentOK := nodeRawCounter(sample.Raw["memory.stat"], name)
			absolute := uint64(16 << 20)
			if name == "sock" {
				absolute = 8 << 20
			}
			allowed := max(absolute, old/20)
			if !oldOK || !currentOK || current > old+allowed {
				return false
			}
		}
		checked++
	}
	return checked == 4
}

func (observer *nodeObserver) waitNewReadiness(ctx context.Context, before [2]int) error {
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		ready := true
		for index, service := range []string{"node1", "node2"} {
			logs, _ := observer.compose(ctx, "logs", "--no-color", service)
			ready = ready && countBytes(logs, []byte(`"state":"READY"`)) > before[index]
		}
		if ready {
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

func verifyNodeStateQuiescence(root string) error {
	entries := 0
	return filepath.WalkDir(filepath.Join(root, "state"), func(path string, entry os.DirEntry, walkErr error) error {
		entries++
		if entries > 512 {
			return errors.New("node state tree exceeds its quiescence bound")
		}
		if walkErr == nil && (strings.HasPrefix(entry.Name(), ".stage-") || strings.HasPrefix(entry.Name(), ".current-")) {
			return errors.New("node state tree retained a temporary resource")
		}
		return walkErr
	})
}
