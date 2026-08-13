package node

import (
	"encoding/json"
	"errors"
	"io"
	"time"
)

type nodeSampleResult struct {
	at        time.Time
	resources []nodeResourceSnapshot
	faults    map[string]bool
}

func classifyNodeSampleErrors(processErr, resourceErr, diagnosticErr error) (error, error) {
	return errors.Join(processErr, resourceErr), diagnosticErr
}

func (observer *nodeObserver) writeNodeSample(result nodeSampleResult) bool {
	observer.observeResources(result.resources, result.faults)
	record := map[string]any{"at": result.at.UTC(), "resources": result.resources, "active_faults": result.faults}
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
