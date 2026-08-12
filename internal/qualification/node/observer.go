package node

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
}

func newNodeObserver(input Campaign) (*nodeObserver, error) {
	samples, err := os.OpenFile(filepath.Join(input.EvidenceRoot, "samples.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(input.EvidenceRoot)))[:12]
	return &nodeObserver{input: input, ctx: ctx, cancel: cancel, clock: [3]bool{true, true, true},
		samples: samples, captured: make(map[string]bool), project: "ardents-node-" + identity,
		imageTag: "run-" + identity, evidenceBad: make(chan struct{}), resources: make(map[string]*nodeResourceSeries)}, nil
}

func (observer *nodeObserver) start() {
	observer.work.Add(3)
	go observer.runClock()
	go observer.runSamples()
	go observer.stopOnEvidenceFailure()
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

func (observer *nodeObserver) runSamples() {
	defer observer.work.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-observer.ctx.Done():
			return
		case at := <-ticker.C:
			ids, idErr := observer.composeBounded(observer.ctx, 32<<10, "ps", "-q")
			arguments := []string{"stats", "--no-stream", "--format", "{{json .}}"}
			for _, id := range strings.Fields(string(ids)) {
				if len(id) >= 12 && len(id) <= 64 {
					arguments = append(arguments, id)
				}
			}
			var stats []byte
			var statsErr error
			if len(arguments) > 4 {
				stats, statsErr = observer.dockerBounded(observer.ctx, 64<<10, 32<<10, arguments...)
			}
			processes, processErr := observer.composeBounded(observer.ctx, 32<<10, "ps", "-a", "--format", "{{.Service}}\t{{.State}}\t{{.ExitCode}}\t{{.ID}}")
			resources, resourceErr := observer.sampleNodeResources(observer.ctx, at)
			observer.observeResources(resources)
			record := map[string]any{"at": at.UTC(), "stats": string(stats), "processes": string(processes), "resources": resources}
			if idErr != nil || statsErr != nil || processErr != nil || resourceErr != nil {
				record["observer_error"] = errors.Join(idErr, statsErr, processErr, resourceErr).Error()
			}
			raw, marshalErr := json.Marshal(record)
			if marshalErr != nil || len(raw) > 128<<10 || observer.sampleCount >= 86464 || observer.sampleBytes+int64(len(raw)+1) > 256<<20 {
				observer.recordEvidenceError(errors.Join(marshalErr, errors.New("node sample evidence exceeded its finite budget")))
				return
			}
			raw = append(raw, '\n')
			written, writeErr := observer.samples.Write(raw)
			if writeErr != nil || written != len(raw) {
				observer.recordEvidenceError(errors.Join(writeErr, io.ErrShortWrite))
				return
			}
			observer.sampleCount++
			observer.sampleBytes += int64(written)
		}
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
