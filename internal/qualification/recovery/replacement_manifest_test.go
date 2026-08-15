package recovery

import (
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyRejectsUncommittedS42InputsAndTruncatedSamples(t *testing.T) {
	for name, mutate := range map[string]func(*replacementCell){
		"manifest digest": func(cell *replacementCell) { cell.CellManifestDigest = "changed" },
		"fault role":      func(cell *replacementCell) { cell.FailureRoles[0] = "responder" },
		"fault offset":    func(cell *replacementCell) { cell.FaultOffsets[0]++ },
		"offered pacing":  func(cell *replacementCell) { cell.ChunkDelayNanos++ },
		"event cardinality": func(cell *replacementCell) {
			cell.Events = append(cell.Events, cell.Events[len(cell.Events)-1])
		},
		"late samples": func(cell *replacementCell) {
			cell.ResourceSamples = cell.ResourceSamples[len(cell.ResourceSamples)-3:]
			cell.ResourceStartedAtNanos = cell.ResourceSamples[0].AtNanos
		},
	} {
		t.Run(name, func(t *testing.T) {
			value := validS42Evidence(t)
			extension := decodeReplacementTest(t, value.S42)
			mutate(&extension.Cells[4])
			var err error
			value.S42, err = json.Marshal(extension)
			if err != nil {
				t.Fatal(err)
			}
			if result := verifyS42Test(value); result.Verdict == "pass" {
				t.Fatalf("uncommitted or truncated S4.2 evidence passed: %+v", result)
			}
		})
	}
}

func TestReplacementResourcesRequireStartToTerminalCoverage(t *testing.T) {
	cell := replacementCell{HostStartedAtNanos: 100, ActiveStartedAtNanos: 101,
		TerminalNanos:          int64(10*time.Minute + time.Second),
		ResourceStartedAtNanos: 1,
		Events:                 []replacementEvent{{LastDeliveryNanos: int64(2 * time.Minute)}},
		HostProcesses:          map[string]processObservationEvidence{"client-app": {ObservedAtNanos: 101}}}
	cell.ResourceSamples = s42Samples(cell.TerminalNanos)
	cell.FinalTraffic = cell.ResourceSamples[len(cell.ResourceSamples)-1]
	if result := verifyReplacementResources(cell); result.Verdict != "pass" {
		t.Fatalf("full resource coverage rejected: %+v", result)
	}
	cell.ResourceSamples = cell.ResourceSamples[len(cell.ResourceSamples)-3:]
	cell.ResourceStartedAtNanos = cell.ResourceSamples[0].AtNanos
	if result := verifyReplacementResources(cell); result.Verdict == "pass" {
		t.Fatalf("terminal-only resource samples passed: %+v", result)
	}
}

func TestReplacementResourcesAcceptRoundedTerminalTraffic(t *testing.T) {
	cell := replacementCell{Bytes: streamBytes, TerminalNanos: int64(4 * time.Second),
		ResourceStartedAtNanos: 1, Events: []replacementEvent{{LastDeliveryNanos: int64(time.Second)}}}
	cell.ResourceSamples = s42Samples(cell.TerminalNanos)
	cell.FinalTraffic = cell.ResourceSamples[len(cell.ResourceSamples)-1]
	cell.FinalTraffic.ClientSent = uint64(cell.Bytes) - 128<<10
	cell.FinalTraffic.PublisherReceived = uint64(cell.Bytes) - 128<<10
	if result := verifyReplacementResources(cell); result.Verdict != "pass" {
		t.Fatalf("rounded terminal traffic rejected: %+v", result)
	}
}

func TestTerminalTrafficSampleAllowsOneIncompleteDockerStatsRound(t *testing.T) {
	terminal := int64(5 * time.Second)
	sample := ResourceSample{AtNanos: int64(3100 * time.Millisecond), ClientSent: 1, PublisherReceived: 1}
	if !terminalTrafficSample(sample, terminal) {
		t.Fatal("complete sample from the prior Docker stats round rejected")
	}
	sample.AtNanos = int64(2999 * time.Millisecond)
	if terminalTrafficSample(sample, terminal) {
		t.Fatal("sample older than two seconds accepted")
	}
}
