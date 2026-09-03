//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Slice-2 outcome signature constants. Honest consumers see two valid
// sources; the probe consumer sees one valid and one forged source whose
// Epoch signature fails the consumer's authority check and is reported as
// "invalid-state". outcomes[2] and outcomes[3] are the production-side
// upstream fallbacks and must always be "not-attempted" for a well-formed
// pilot run; the verify step rejects any deviation here.
const (
	outcomeValid        = "valid"
	outcomeInvalidState = "invalid-state"
	outcomeNotAttempted = "not-attempted"
)

var (
	honestOutcomes = [4]string{outcomeValid, outcomeValid, outcomeNotAttempted, outcomeNotAttempted}
	probeOutcomes  = [4]string{outcomeValid, outcomeInvalidState, outcomeNotAttempted, outcomeNotAttempted}
)

// AdversaryVerdict is the slice-2 outcome of the adversarial convergence
// check. Five honest consumers must see two valid sources each, the one
// probe consumer must see one valid + one non-valid source, all six must
// converge on the same content-addressed generation, and the per-node id
// assignment must match the spec: node-1..5 are honest and node-6 is the
// probe. The fields count the three classes of pre-count deviation so the
// rejection reason can name the offending report.
type AdversaryVerdict struct {
	StartedAt       time.Time
	CompletedAt     time.Time
	Accept          bool
	Reason          string
	HonestNodeCount int
	ProbeNodeCount  int
	DistinctResults int
	Generation      string
	Honest          []NodeAdversaryReport
	Probe           []NodeAdversaryReport

	// GenerationMismatchCount is the number of reports whose parsed
	// generation does not equal the expected generation. A generation
	// divergence is the canonical "consumers did not converge on the
	// prebake State" signal and must be reported even when the per-source
	// outcome counts look right; without this field six nodes could
	// silently converge on a wrong generation and still get accept=true.
	GenerationMismatchCount int

	// MismatchedSignatureCount is the number of reports whose
	// source_outcomes array does not match the honest or the probe
	// signature exactly. Any deviation in outcomes[2] or outcomes[3]
	// (e.g. "valid" instead of "not-attempted") or in outcomes[1] for
	// the probe ("framing-failed" instead of "invalid-state") counts
	// here.
	MismatchedSignatureCount int

	// ParseErrorCount is the number of reports whose node log could not
	// be parsed into a source-wave-accepted event.
	ParseErrorCount int
}

// NodeAdversaryReport is one consumer's slice-2 view: generation,
// per-source outcomes, and a boolean flag indicating whether this node is
// classified as honest (both configured sources valid) or probe (exactly
// one valid, one invalid-state). GenerationMatches is true iff the
// reported generation equals the expected generation; the per-node loop
// sets it so the count and the per-node table both surface divergences.
type NodeAdversaryReport struct {
	NodeID            string
	Generation        string
	GenerationMatches bool
	SourceOutcomes    [4]string
	Honest            bool
	Probe             bool
	ParseError        string
}

// VerifyAdversaryScenario is the slice-2 acceptance check. It classifies
// each per-node event by an exact-match check on the four source_outcomes
// values and rejects any deviation:
//   - node-1..5 are honest iff [valid, valid, not-attempted, not-attempted]
//   - node-6 is the probe iff [valid, invalid-state, not-attempted, not-attempted]
//
// Convergence is checked across the full set: all six nodes must report
// the expected generation. The acceptance criteria:
//   - exactly 6 reports
//   - all 6 generations equal the expected generation
//   - exactly 5 honest and 1 probe by exact signature match
//   - distinct=1
//   - the probe is node-6 and the honest set is exactly node-1..5
//
// Any deviation produces accept=false with a specific reason that names
// the offending node id and the offending values. The verdict is written
// to pilot-adversary-verdict.{md,json} so a human reviewer can read the
// per-node table.
func VerifyAdversaryScenario(evidenceDir, expectedGeneration string) (AdversaryVerdict, error) {
	if evidenceDir == "" {
		return AdversaryVerdict{}, errors.New("pilot: evidence dir is empty")
	}
	if expectedGeneration == "" {
		return AdversaryVerdict{}, errors.New("pilot: expected generation is empty")
	}
	startedAt := time.Now().UTC()
	nodesDir := filepath.Join(evidenceDir, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		return AdversaryVerdict{}, fmt.Errorf("pilot: read nodes dir: %w", err)
	}
	reports := make([]NodeAdversaryReport, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "node-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		report := NodeAdversaryReport{NodeID: strings.TrimSuffix(strings.TrimPrefix(name, "node-"), ".json")}
		event, err := ReadSourceWaveEvent(filepath.Join(nodesDir, name))
		if err != nil {
			report.ParseError = err.Error()
		} else {
			report.Generation = event.Generation
			report.SourceOutcomes = event.SourceOutcomes
			report.Honest, report.Probe = classifyAdversaryNode(event.SourceOutcomes)
			report.GenerationMatches = event.Generation == expectedGeneration
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].NodeID < reports[j].NodeID })

	verdict := AdversaryVerdict{StartedAt: startedAt, CompletedAt: time.Now().UTC(), Generation: expectedGeneration}
	distinct := map[string]int{}
	for _, r := range reports {
		if r.ParseError == "" {
			distinct[r.Generation]++
			if !r.GenerationMatches {
				verdict.GenerationMismatchCount++
			}
			if !r.Honest && !r.Probe {
				verdict.MismatchedSignatureCount++
			}
		} else {
			verdict.ParseErrorCount++
		}
		switch {
		case r.Honest:
			verdict.Honest = append(verdict.Honest, r)
		case r.Probe:
			verdict.Probe = append(verdict.Probe, r)
		}
	}
	verdict.HonestNodeCount = len(verdict.Honest)
	verdict.ProbeNodeCount = len(verdict.Probe)
	verdict.DistinctResults = len(distinct)

	switch {
	case len(reports) != 6:
		verdict.Accept = false
		verdict.Reason = fmt.Sprintf("expected 6 node reports, found %d", len(reports))
	case verdict.GenerationMismatchCount > 0:
		verdict.Accept = false
		verdict.Reason = adversaryGenerationMismatchReason(reports, expectedGeneration)
	case verdict.MismatchedSignatureCount > 0:
		verdict.Accept = false
		verdict.Reason = adversarySignatureMismatchReason(reports)
	case verdict.ParseErrorCount > 0:
		verdict.Accept = false
		verdict.Reason = adversaryParseErrorReason(reports)
	case verdict.HonestNodeCount != 5:
		verdict.Accept = false
		verdict.Reason = fmt.Sprintf("expected 5 honest nodes, found %d (probe=%d, mismatched-signature=%d, parse-error=%d)",
			verdict.HonestNodeCount, verdict.ProbeNodeCount, verdict.MismatchedSignatureCount, verdict.ParseErrorCount)
	case verdict.ProbeNodeCount != 1:
		verdict.Accept = false
		verdict.Reason = fmt.Sprintf("expected 1 probe node, found %d (honest=%d, mismatched-signature=%d, parse-error=%d)",
			verdict.ProbeNodeCount, verdict.HonestNodeCount, verdict.MismatchedSignatureCount, verdict.ParseErrorCount)
	case verdict.DistinctResults != 1:
		verdict.Accept = false
		verdict.Reason = fmt.Sprintf("consumers diverged: %d distinct result sets", verdict.DistinctResults)
	case !specificAdversaryNodeIDsOK(verdict):
		verdict.Accept = false
		verdict.Reason = specificAdversaryNodeIDsReason(verdict)
	default:
		verdict.Accept = true
		verdict.Reason = "5 honest + 1 probe converged on the same generation; probe rejected the forged source"
	}
	if err := writeAdversaryVerdictArtifacts(evidenceDir, verdict); err != nil {
		return verdict, fmt.Errorf("pilot: write adversary verdict artefacts: %w", err)
	}
	return verdict, nil
}

// classifyAdversaryNode is the slice-2 outcome signature: an exact match
// on the full [4]string of source outcomes. The honest signature is
// [valid, valid, not-attempted, not-attempted]; the probe signature is
// [valid, invalid-state, not-attempted, not-attempted]. Any other shape
// — including a probe with "framing-failed" or an honest node with
// outcomes[2] = "valid" — is neither honest nor probe and is rejected by
// the verify step's count and signature checks.
func classifyAdversaryNode(outcomes [4]string) (honest, probe bool) {
	if outcomes == honestOutcomes {
		return true, false
	}
	if outcomes == probeOutcomes {
		return false, true
	}
	return false, false
}

// adversaryGenerationMismatchReason picks the first report whose parsed
// generation differs from the expected one and reports its id and both
// values. Reports are pre-sorted by NodeID so the "first" is
// deterministic and matches the per-node table in the verdict artefact.
func adversaryGenerationMismatchReason(reports []NodeAdversaryReport, expected string) string {
	for _, r := range reports {
		if r.ParseError == "" && !r.GenerationMatches {
			return fmt.Sprintf("node-%s reports generation %q, expected %q",
				r.NodeID, r.Generation, expected)
		}
	}
	return fmt.Sprintf("generation mismatch count is non-zero but no offending report found (expected %q)", expected)
}

// adversarySignatureMismatchReason picks the first report whose
// source_outcomes array matches neither the honest nor the probe
// signature and reports its id, its actual outcomes, and the two
// expected signatures. This is the rejection reason a human reviewer
// sees when a node produced a non-canonical outcomes array — e.g. the
// probe saw "framing-failed" instead of "invalid-state", or an honest
// node reported outcomes[2] = "valid" instead of "not-attempted".
func adversarySignatureMismatchReason(reports []NodeAdversaryReport) string {
	for _, r := range reports {
		if r.ParseError == "" && !r.Honest && !r.Probe {
			return fmt.Sprintf("node-%s source_outcomes %v do not match honest signature %v or probe signature %v",
				r.NodeID, r.SourceOutcomes, honestOutcomes, probeOutcomes)
		}
	}
	return "mismatched-signature count is non-zero but no offending report found"
}

// adversaryParseErrorReason picks the first report whose node log could
// not be parsed into a source-wave-accepted event and reports its id
// and the underlying error. The error string comes from
// ReadSourceWaveEvent (typically "pilot: no source-wave-accepted event
// in node log" or "pilot: parse node log line: ...") so the human
// reviewer can tell missing-event from malformed-JSON failures.
func adversaryParseErrorReason(reports []NodeAdversaryReport) string {
	for _, r := range reports {
		if r.ParseError != "" {
			return fmt.Sprintf("node-%s could not be parsed: %s", r.NodeID, r.ParseError)
		}
	}
	return "parse-error count is non-zero but no offending report found"
}

// specificAdversaryNodeIDsOK enforces the spec invariant that node-1..5
// are the honest consumers and node-6 is the probe. Counts are already
// checked above; this is the second-pass identity check that catches a
// swap between an honest and a probe node (e.g. node-3 with probe
// outcomes and node-6 with honest outcomes). NodeID is the bare numeric
// suffix the per-node loop stores (see the NodeAdversaryReport field
// doc), so the comparison is against the string "1".."6" without the
// "node-" prefix the on-disk filename carries.
func specificAdversaryNodeIDsOK(verdict AdversaryVerdict) bool {
	if len(verdict.Probe) != 1 || verdict.Probe[0].NodeID != "6" {
		return false
	}
	honestSet := map[string]struct{}{}
	for _, r := range verdict.Honest {
		honestSet[r.NodeID] = struct{}{}
	}
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		if _, ok := honestSet[id]; !ok {
			return false
		}
	}
	return true
}

// specificAdversaryNodeIDsReason returns a reason naming the offending
// node id and the expected assignment. Reports are pre-sorted by NodeID
// so the first deviation is the smallest one.
func specificAdversaryNodeIDsReason(verdict AdversaryVerdict) string {
	if len(verdict.Probe) != 1 {
		return fmt.Sprintf("expected probe to be exactly node-6, found %d probe reports", len(verdict.Probe))
	}
	if verdict.Probe[0].NodeID != "6" {
		return fmt.Sprintf("probe is node-%s, expected node-6", verdict.Probe[0].NodeID)
	}
	honestSet := map[string]struct{}{}
	for _, r := range verdict.Honest {
		honestSet[r.NodeID] = struct{}{}
	}
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		if _, ok := honestSet[id]; !ok {
			return fmt.Sprintf("node-%s expected to be honest but is missing from honest list", id)
		}
	}
	return "honest/probe node id assignment does not match spec (node-1..5 honest, node-6 probe)"
}

func writeAdversaryVerdictArtifacts(evidenceDir string, verdict AdversaryVerdict) error {
	marshaled, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal adversary verdict: %w", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "pilot-adversary-convergence.json"),
		append(marshaled, '\n'), 0o600); err != nil {
		return fmt.Errorf("write pilot-adversary-convergence.json: %w", err)
	}
	lines := []string{
		"# Multi-node pilot adversary verdict",
		"",
		fmt.Sprintf("- Started: %s", verdict.StartedAt.Format(time.RFC3339Nano)),
		fmt.Sprintf("- Completed: %s", verdict.CompletedAt.Format(time.RFC3339Nano)),
		fmt.Sprintf("- Accept: %v", verdict.Accept),
		fmt.Sprintf("- Reason: %s", verdict.Reason),
		fmt.Sprintf("- Expected generation: %s", verdict.Generation),
		fmt.Sprintf("- Honest nodes: %d", verdict.HonestNodeCount),
		fmt.Sprintf("- Probe nodes: %d", verdict.ProbeNodeCount),
		fmt.Sprintf("- Distinct result sets: %d", verdict.DistinctResults),
		fmt.Sprintf("- Generation mismatches: %d", verdict.GenerationMismatchCount),
		fmt.Sprintf("- Mismatched signatures: %d", verdict.MismatchedSignatureCount),
		fmt.Sprintf("- Parse errors: %d", verdict.ParseErrorCount),
		"",
		"## Honest consumers",
		"",
		"| node | generation | matches | sources |",
		"|---|---|---|---|",
	}
	for _, r := range verdict.Honest {
		lines = append(lines, fmt.Sprintf("| %s | %s | %t | [%s %s %s %s] |",
			r.NodeID, r.Generation, r.GenerationMatches,
			r.SourceOutcomes[0], r.SourceOutcomes[1],
			r.SourceOutcomes[2], r.SourceOutcomes[3]))
	}
	lines = append(lines, "", "## Probe consumers", "", "| node | generation | matches | sources | rejected |", "|---|---|---|---|---|")
	for _, r := range verdict.Probe {
		rejected := second(r.SourceOutcomes)
		lines = append(lines, fmt.Sprintf("| %s | %s | %t | [%s %s %s %s] | %s |",
			r.NodeID, r.Generation, r.GenerationMatches,
			r.SourceOutcomes[0], r.SourceOutcomes[1],
			r.SourceOutcomes[2], r.SourceOutcomes[3], rejected))
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "pilot-adversary-verdict.md"),
		[]byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("write pilot-adversary-verdict.md: %w", err)
	}
	return nil
}

func second(outcomes [4]string) string {
	if outcomes[1] != "valid" {
		return outcomes[1]
	}
	return outcomes[0]
}
