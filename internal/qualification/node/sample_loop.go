package node

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type nodeSampleResult struct {
	sequence      int
	at            time.Time
	processes     []byte
	resources     []nodeResourceSnapshot
	diagnostics   []byte
	faults        map[string]bool
	diagnosticErr error
	err           error
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
			go observer.collectNodeSample(sequence, at, observer.faultSnapshot(), results)
			sequence++
		}
	}
}

func (observer *nodeObserver) collectNodeSample(sequence int, at time.Time, faults map[string]bool, output chan<- nodeSampleResult) {
	ctx, cancel := context.WithTimeout(observer.ctx, 12*time.Second)
	defer cancel()
	processes, processErr := observer.composeBounded(ctx, 32<<10, "ps", "-a", "--format",
		"{{.Service}}\t{{.State}}\t{{.ExitCode}}\t{{.ID}}")
	resources, resourceErr := observer.sampleNodeResources(ctx, at)
	diagnostics, diagnosticErr := observer.composeBounded(ctx, 256<<10, "logs", "--no-color", "--timestamps", "--since", "2s",
		"source1", "source2", "endpoint", "node1", "node2")
	fatalErr, diagnosticErr := classifyNodeSampleErrors(processErr, resourceErr, diagnosticErr)
	result := nodeSampleResult{sequence: sequence, at: at, processes: processes, resources: resources,
		diagnostics: diagnostics, faults: faults, diagnosticErr: diagnosticErr, err: fatalErr}
	select {
	case output <- result:
	case <-observer.ctx.Done():
	}
}

func classifyNodeSampleErrors(processErr, resourceErr, diagnosticErr error) (error, error) {
	return errors.Join(processErr, resourceErr), diagnosticErr
}

func (observer *nodeObserver) writeNodeSample(result nodeSampleResult) bool {
	observer.observeResources(result.resources, result.faults)
	record := map[string]any{"at": result.at.UTC(), "processes": string(result.processes), "resources": result.resources,
		"candidate_events": string(result.diagnostics), "active_faults": result.faults}
	if result.diagnosticErr != nil {
		record["candidate_diagnostic_error"] = result.diagnosticErr.Error()
	}
	if result.err != nil {
		record["observer_error"] = result.err.Error()
		observer.recordEvidenceError(result.err)
	}
	raw, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		observer.recordEvidenceError(marshalErr)
		return false
	}
	if len(raw) > 128<<10 || observer.sampleCount >= observer.sampleLimit ||
		observer.sampleBytes+int64(len(raw)+1) > observer.sampleBudget {
		observer.recordEvidenceError(errors.New("node sample evidence exceeded its finite budget"))
		return false
	}
	raw = append(raw, '\n')
	written, writeErr := observer.samples.Write(raw)
	if writeErr != nil {
		observer.recordEvidenceError(writeErr)
		return false
	}
	if written != len(raw) {
		observer.recordEvidenceError(io.ErrShortWrite)
		return false
	}
	observer.sampleCount++
	observer.sampleBytes += int64(written)
	return true
}
