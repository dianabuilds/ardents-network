package node

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type nodeSampleResult struct {
	sequence  int
	at        time.Time
	processes []byte
	resources []nodeResourceSnapshot
	err       error
}

func (observer *nodeObserver) runSamples() {
	defer observer.work.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	results := make(chan nodeSampleResult, 8)
	active, sequence, next := 0, 0, 0
	pending := make(map[int]nodeSampleResult, cap(results))
	for {
		select {
		case <-observer.ctx.Done():
			return
		case result := <-results:
			active--
			pending[result.sequence] = result
			for {
				ordered, found := pending[next]
				if !found {
					break
				}
				delete(pending, next)
				next++
				if !observer.writeNodeSample(ordered) {
					return
				}
			}
		case at := <-ticker.C:
			if active >= cap(results) {
				observer.recordEvidenceError(errors.New("node one-second observer concurrency bound was exhausted"))
				return
			}
			active++
			go observer.collectNodeSample(sequence, at, results)
			sequence++
		}
	}
}

func (observer *nodeObserver) collectNodeSample(sequence int, at time.Time, output chan<- nodeSampleResult) {
	ctx, cancel := context.WithTimeout(observer.ctx, 12*time.Second)
	defer cancel()
	processes, processErr := observer.composeBounded(ctx, 32<<10, "ps", "-a", "--format",
		"{{.Service}}\t{{.State}}\t{{.ExitCode}}\t{{.ID}}")
	resources, resourceErr := observer.sampleNodeResources(ctx, at)
	result := nodeSampleResult{sequence: sequence, at: at, processes: processes, resources: resources,
		err: errors.Join(processErr, resourceErr)}
	select {
	case output <- result:
	case <-observer.ctx.Done():
	}
}

func (observer *nodeObserver) writeNodeSample(result nodeSampleResult) bool {
	observer.observeResources(result.resources)
	record := map[string]any{"at": result.at.UTC(), "processes": string(result.processes), "resources": result.resources}
	if result.err != nil {
		record["observer_error"] = result.err.Error()
		observer.recordEvidenceError(result.err)
	}
	raw, marshalErr := json.Marshal(record)
	if marshalErr != nil || len(raw) > 128<<10 || observer.sampleCount >= observer.sampleLimit ||
		observer.sampleBytes+int64(len(raw)+1) > observer.sampleBudget {
		observer.recordEvidenceError(errors.Join(marshalErr, errors.New("node sample evidence exceeded its finite budget")))
		return false
	}
	raw = append(raw, '\n')
	written, writeErr := observer.samples.Write(raw)
	if writeErr != nil || written != len(raw) {
		observer.recordEvidenceError(errors.Join(writeErr, io.ErrShortWrite))
		return false
	}
	observer.sampleCount++
	observer.sampleBytes += int64(written)
	return true
}
