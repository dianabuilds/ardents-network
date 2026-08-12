package node

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type nodeObserver struct {
	input        Campaign
	ctx          context.Context
	cancel       context.CancelFunc
	work         sync.WaitGroup
	mu           sync.Mutex
	clock        [3]bool
	samples      *os.File
	cleanupOnce  sync.Once
	cleanupErr   error
	captured     map[string]bool
	evidenceErr  error
	evidenceBad  chan struct{}
	evidenceOnce sync.Once
	project      string
	imageTag     string
	sampleBytes  int64
	sampleCount  int
	resources    map[string]*nodeResourceSeries
	sourceDigest string
	collectorID  string
	sampleLimit  int
	sampleBudget int64
}

func newNodeObserver(input Campaign) (*nodeObserver, error) {
	sourceDigest, err := captureNodeSourceIdentity(input.ComposeFile, input.EvidenceRoot)
	if err != nil {
		return nil, err
	}
	samples, err := os.OpenFile(filepath.Join(input.EvidenceRoot, "samples.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(input.EvidenceRoot)))[:12]
	limit, budget := nodeSampleBounds(input.Mode)
	return &nodeObserver{input: input, ctx: ctx, cancel: cancel, clock: [3]bool{true, true, true}, sourceDigest: sourceDigest,
		samples: samples, captured: make(map[string]bool), project: "ardents-node-" + identity,
		imageTag: "run-" + identity, evidenceBad: make(chan struct{}), resources: make(map[string]*nodeResourceSeries),
		sampleLimit: limit, sampleBudget: budget}, nil
}

func (observer *nodeObserver) start() {
	observer.work.Add(2)
	go observer.runClock()
	go observer.stopOnEvidenceFailure()
}

func (observer *nodeObserver) startSamples() {
	observer.work.Add(1)
	go observer.runSamples()
}

func (observer *nodeObserver) stopOnEvidenceFailure() {
	defer observer.work.Done()
	select {
	case <-observer.ctx.Done():
		return
	case <-observer.evidenceBad:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := observer.compose(ctx, "kill", "node1", "node2")
	observer.recordEvidenceError(err)
}

func (observer *nodeObserver) runClock() {
	defer observer.work.Done()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		observer.mu.Lock()
		enabled := observer.clock
		observer.mu.Unlock()
		now := time.Now()
		for index, zone := range []string{"e", "n1", "n2"} {
			if enabled[index] {
				observer.recordEvidenceError(os.Chtimes(filepath.Join(observer.input.FixtureRoot, "clock", zone+".observation"), now, now))
			}
		}
		select {
		case <-observer.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func nodeSampleBounds(mode string) (int, int64) {
	switch mode {
	case "churn-2h":
		return 8_000, int64(2) << 30
	case "unattended-24h":
		return 87_000, int64(8) << 30
	default:
		return 4_096, int64(512) << 20
	}
}

func (observer *nodeObserver) composeBounded(ctx context.Context, limit int, arguments ...string) ([]byte, error) {
	args := append([]string{"compose", "-p", observer.project, "-f", observer.input.ComposeFile}, arguments...)
	return observer.dockerBounded(ctx, limit, 32<<10, args...)
}

func (observer *nodeObserver) setClock(index int, enabled bool) {
	observer.mu.Lock()
	observer.clock[index] = enabled
	observer.mu.Unlock()
}

func (observer *nodeObserver) waitReady(ctx context.Context, timeout time.Duration) error {
	return observer.waitServiceReady(ctx, timeout, "node1", "node2")
}

func (observer *nodeObserver) waitServiceReady(ctx context.Context, timeout time.Duration, services ...string) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		logs, _ := observer.compose(ctx, append([]string{"logs", "--no-color"}, services...)...)
		if countBytes(logs, []byte(`"state":"READY"`)) >= len(services) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("node candidates did not reach READY")
		case <-ticker.C:
		}
	}
}

func (observer *nodeObserver) waitServiceRunning(ctx context.Context, timeout time.Duration, service string) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if observer.running(ctx, service) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("node candidate did not remain running")
		case <-ticker.C:
		}
	}
}

func countBytes(raw, target []byte) int {
	count := 0
	for index := 0; index+len(target) <= len(raw); {
		if string(raw[index:index+len(target)]) == string(target) {
			count++
			index += len(target)
		} else {
			index++
		}
	}
	return count
}

func (observer *nodeObserver) close() {
	observer.cancel()
	observer.work.Wait()
	observer.recordEvidenceError(observer.samples.Close())
}
