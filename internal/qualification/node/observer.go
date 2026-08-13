package node

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type nodeObserver struct {
	input              Campaign
	composeFile        string
	ctx                context.Context
	cancel             context.CancelFunc
	work               sync.WaitGroup
	mu                 sync.Mutex
	clock              [3]bool
	samples            *os.File
	cleanupOnce        sync.Once
	cleanupErr         error
	captured           map[string]bool
	evidenceErr        error
	evidenceBad        chan struct{}
	evidenceOnce       sync.Once
	project            string
	imageTag           string
	sampleBytes        int64
	sampleCount        int
	resources          map[string]*nodeResourceSeries
	activeFaults       map[string]bool
	resourceReset      chan nodeResourceReset
	sourceDigest       string
	sourceRoot         string
	initialStateDigest string
	collectorID        string
	sampleLimit        int
	sampleBudget       int64
}

type nodeServiceIdentity struct {
	service string
	id      string
}

func newNodeObserver(input Campaign) (*nodeObserver, error) {
	sourceDigest, sourceRoot, err := captureNodeSourceIdentity(input.ComposeFile, input.EvidenceRoot)
	if err != nil {
		return nil, err
	}
	samples, err := os.OpenFile(filepath.Join(input.EvidenceRoot, "samples.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(input.EvidenceRoot)))[:12]
	mode, _ := selectNodeCampaignMode(input.Mode)
	return &nodeObserver{input: input, composeFile: input.ComposeFile, ctx: ctx, cancel: cancel, clock: [3]bool{true, true, true},
		sourceDigest: sourceDigest, sourceRoot: sourceRoot,
		samples: samples, captured: make(map[string]bool), project: "ardents-node-" + identity,
		imageTag: "run-" + identity, evidenceBad: make(chan struct{}), resources: make(map[string]*nodeResourceSeries),
		activeFaults: make(map[string]bool), resourceReset: make(chan nodeResourceReset, 32),
		sampleLimit: mode.sampleLimit, sampleBudget: mode.sampleBudget}, nil
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

func (observer *nodeObserver) composeBounded(ctx context.Context, limit int, arguments ...string) ([]byte, error) {
	args := append([]string{"compose", "-p", observer.project, "-f", observer.composeFile}, arguments...)
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
	identities := make([]nodeServiceIdentity, 0, len(services))
	for _, service := range services {
		identity, err := observer.serviceID(ctx, service)
		if err != nil {
			return err
		}
		identities = append(identities, nodeServiceIdentity{service: service, id: identity})
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		ready, err := observer.serviceSetReady(ctx, identities)
		if err != nil {
			return err
		}
		if ready {
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

func (observer *nodeObserver) serviceSetReady(ctx context.Context, identities []nodeServiceIdentity) (bool, error) {
	for _, expected := range identities {
		current, err := observer.serviceID(ctx, expected.service)
		if err != nil {
			return false, err
		}
		if current != expected.id {
			return false, invalidNodeCampaign(errors.New("node candidate identity changed while awaiting readiness"))
		}
		logs, err := observer.dockerBounded(ctx, 256<<10, 32<<10, "logs", "--since", "45s", current)
		if err != nil {
			return false, err
		}
		if !nodeReadyEvent(logs) {
			return false, nil
		}
	}
	return true, nil
}

func nodeReadyEvent(logs []byte) bool {
	return countNodeLogEvents(logs, "", "READY") > 0
}

func (observer *nodeObserver) waitServiceRunning(ctx context.Context, timeout time.Duration, service string) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		running, err := observer.running(ctx, service)
		if err != nil {
			return err
		}
		if running {
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

func countNodeLogEvents(raw []byte, kind, state string) int {
	count := 0
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		var event struct {
			Kind  string `json:"kind"`
			State string `json:"state"`
		}
		if json.Unmarshal(line, &event) == nil && event.State == state && (kind == "" || event.Kind == kind) {
			count++
		}
	}
	return count
}

func nodeLogContainsExactLine(raw []byte, expected string) bool {
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if string(bytes.TrimSpace(line)) == expected {
			return true
		}
	}
	return false
}
