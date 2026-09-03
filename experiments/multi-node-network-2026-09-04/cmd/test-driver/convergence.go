//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SourceWaveEvent is the single JSON event the source-client CLI emits on
// every accepted wave. The pilot treats the contents of this object as the
// canonical answer to "did this consumer converge on the published State?".
type SourceWaveEvent struct {
	Schema             string    `json:"schema"`
	Kind               string    `json:"kind"`
	Generation         string    `json:"generation"`
	Epoch              uint64    `json:"epoch"`
	SourceAttempts     uint16    `json:"source_attempts"`
	SourceOutcomes     [4]string `json:"source_outcomes"`
	LatestCompleteness string    `json:"latest_completeness"`
}

// NodeReport is the per-consumer summary the verify step writes for the
// human-readable pilot-verdict.md and the machine-readable
// pilot-convergence.json artefacts. NodeID is the deterministic container
// name the compose file uses (node-1..node-6).
type NodeReport struct {
	NodeID             string    `json:"node_id"`
	Generation         string    `json:"generation"`
	Epoch              uint64    `json:"epoch"`
	SourceAttempts     uint16    `json:"source_attempts"`
	SourceOutcomes     [4]string `json:"source_outcomes"`
	LatestCompleteness string    `json:"latest_completeness"`
	ObservedAt         time.Time `json:"observed_at"`
	ParseError         string    `json:"parse_error,omitempty"`
}

// Verdict is the aggregate outcome of the verify step. It is what the
// test-driver container exits with and what the human reviewer reads in
// pilot-verdict.md.
type Verdict struct {
	StartedAt       time.Time    `json:"started_at"`
	CompletedAt     time.Time    `json:"completed_at"`
	Accept          bool         `json:"accept"`
	Reason          string       `json:"reason"`
	ExpectedDigest  string       `json:"expected_generation"`
	Nodes           []NodeReport `json:"nodes"`
	DistinctResults int          `json:"distinct_results"`
}

// ReadSourceWaveEvent extracts exactly one SourceWaveEvent from a node log
// file. The source-client CLI emits a small prelude plus one event; this
// reader is tolerant of additional log lines after the event, but it is
// strict about finding exactly one source-wave-accepted event in the file.
func ReadSourceWaveEvent(path string) (SourceWaveEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return SourceWaveEvent{}, fmt.Errorf("pilot: open node log: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return SourceWaveEvent{}, fmt.Errorf("pilot: read node log: %w", err)
		}
		trimmed := bytesTrimRight(line)
		if len(trimmed) == 0 {
			if err == io.EOF {
				return SourceWaveEvent{}, errors.New("pilot: no source-wave-accepted event in node log")
			}
			continue
		}
		var event SourceWaveEvent
		if err := json.Unmarshal(trimmed, &event); err != nil {
			if err == io.EOF {
				continue
			}
			return SourceWaveEvent{}, fmt.Errorf("pilot: parse node log line: %w", err)
		}
		if event.Schema == "ardents-source-event-v1" && event.Kind == "source-wave-accepted" {
			return event, nil
		}
		if err == io.EOF {
			return SourceWaveEvent{}, errors.New("pilot: no source-wave-accepted event in node log")
		}
	}
}

func bytesTrimRight(in []byte) []byte {
	end := len(in)
	for end > 0 && (in[end-1] == '\n' || in[end-1] == '\r' || in[end-1] == ' ' || in[end-1] == '\t') {
		end--
	}
	return in[:end]
}

// VerifyConvergence reads every per-node log under evidenceDir, asserts that
// all six consumers saw the same source-wave-accepted event, and writes the
// machine-readable and human-readable artefacts the run produces. The
// expected generation is what the prebake step wrote into the State
// fixture; it is passed in so the verify step is independent of the
// prebake binary.
func VerifyConvergence(evidenceDir, expectedGeneration string) (Verdict, error) {
	if evidenceDir == "" {
		return Verdict{}, errors.New("pilot: evidence dir is empty")
	}
	if expectedGeneration == "" {
		return Verdict{}, errors.New("pilot: expected generation is empty")
	}
	startedAt := time.Now().UTC()
	nodesDir := filepath.Join(evidenceDir, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		return Verdict{}, fmt.Errorf("pilot: read nodes dir: %w", err)
	}
	reports := make([]NodeReport, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "node-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		report := NodeReport{NodeID: strings.TrimSuffix(strings.TrimPrefix(name, "node-"), ".json"),
			ObservedAt: time.Now().UTC()}
		event, err := ReadSourceWaveEvent(filepath.Join(nodesDir, name))
		if err != nil {
			report.ParseError = err.Error()
		} else {
			report.Generation = event.Generation
			report.Epoch = event.Epoch
			report.SourceAttempts = event.SourceAttempts
			report.SourceOutcomes = event.SourceOutcomes
			report.LatestCompleteness = event.LatestCompleteness
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].NodeID < reports[j].NodeID })

	distinct := map[string]int{}
	for _, report := range reports {
		key := fmt.Sprintf("gen=%s epoch=%d attempts=%d outcomes=%v completeness=%s err=%s",
			report.Generation, report.Epoch, report.SourceAttempts, report.SourceOutcomes,
			report.LatestCompleteness, report.ParseError)
		distinct[key]++
	}

	verdict := Verdict{StartedAt: startedAt, CompletedAt: time.Now().UTC(), ExpectedDigest: expectedGeneration,
		Nodes: reports, DistinctResults: len(distinct)}
	switch {
	case len(reports) < 6:
		verdict.Accept = false
		verdict.Reason = fmt.Sprintf("only %d node reports found, expected 6", len(reports))
	case distinctResultsHaveParseError(distinct):
		verdict.Accept = false
		verdict.Reason = "one or more nodes failed to emit a source-wave-accepted event"
	case len(distinct) != 1:
		verdict.Accept = false
		verdict.Reason = fmt.Sprintf("consumers diverged: %d distinct result sets across %d nodes", len(distinct), len(reports))
	default:
		first := reports[0]
		if first.Generation != expectedGeneration {
			verdict.Accept = false
			verdict.Reason = fmt.Sprintf("converged generation %q does not match prebake generation %q",
				first.Generation, expectedGeneration)
		} else {
			verdict.Accept = true
			verdict.Reason = "all 6 consumers converged on the same source-wave-accepted event"
		}
	}
	if err := writeVerdictArtifacts(evidenceDir, verdict); err != nil {
		return verdict, fmt.Errorf("pilot: write verdict artefacts: %w", err)
	}
	return verdict, nil
}

func distinctResultsHaveParseError(distinct map[string]int) bool {
	for key := range distinct {
		if strings.Contains(key, "err=pilot:") {
			return true
		}
	}
	return false
}

func writeVerdictArtifacts(evidenceDir string, verdict Verdict) error {
	marshaled, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "pilot-convergence.json"),
		append(marshaled, '\n'), 0o600); err != nil {
		return err
	}
	lines := []string{
		"# Multi-node pilot verdict",
		"",
		fmt.Sprintf("- Started: %s", verdict.StartedAt.Format(time.RFC3339Nano)),
		fmt.Sprintf("- Completed: %s", verdict.CompletedAt.Format(time.RFC3339Nano)),
		fmt.Sprintf("- Accept: %v", verdict.Accept),
		fmt.Sprintf("- Reason: %s", verdict.Reason),
		fmt.Sprintf("- Expected generation: %s", verdict.ExpectedDigest),
		fmt.Sprintf("- Distinct result sets: %d", verdict.DistinctResults),
		fmt.Sprintf("- Nodes observed: %d", len(verdict.Nodes)),
		"",
		"| node | generation | epoch | attempts | outcomes | completeness | error |",
		"|---|---|---|---|---|---|---|",
	}
	for _, node := range verdict.Nodes {
		lines = append(lines, fmt.Sprintf("| %s | %s | %d | %d | %v | %s | %s |",
			node.NodeID, node.Generation, node.Epoch, node.SourceAttempts,
			node.SourceOutcomes, node.LatestCompleteness, node.ParseError))
	}
	return os.WriteFile(filepath.Join(evidenceDir, "pilot-verdict.md"),
		[]byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
