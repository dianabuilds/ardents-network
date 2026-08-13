package node

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestHostResourceStreamUsesCollectorScheduleWithoutPerTickStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	schedule := make(chan time.Time, 3)
	base := time.Unix(1_800_000_000, 0)
	for offset := range 3 {
		schedule <- base.Add(time.Duration(offset+1) * time.Second)
	}
	var sampled []time.Time
	var output bytes.Buffer
	err := streamHostResources(ctx, &output, schedule, func(at time.Time) ([]byte, error) {
		sampled = append(sampled, at)
		if len(sampled) == 3 {
			cancel()
		}
		return []byte(`[]`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sampled) != 3 || !sampled[0].Equal(base.Add(time.Second)) ||
		!sampled[2].Equal(base.Add(3*time.Second)) {
		t.Fatalf("sample schedule = %v", sampled)
	}
	if output.String() != "[]\n[]\n[]\n" {
		t.Fatalf("stream output = %q", output.String())
	}
}

func TestResourceStreamEchoesAppliedCandidateGeneration(t *testing.T) {
	candidates := []nodeHostCandidate{{Service: "node1", ContainerID: "123456789012", PID: 42}}
	payload, err := encodeNodeResourceStreamUpdate(7, candidates)
	if err != nil {
		t.Fatal(err)
	}
	update, err := decodeNodeResourceStreamUpdate(string(payload))
	if err != nil {
		t.Fatal(err)
	}
	if update.Generation != 7 || len(update.Candidates) != 1 || update.Candidates[0].PID != 42 {
		t.Fatalf("decoded update = %+v", update)
	}
}

func TestResourceFaultActivationPrecedesGenerationAcknowledgement(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	transitions := []nodeFaultTransition{
		{generation: 2, at: base, faults: map[string]bool{"absence:source1": true}},
		{generation: 3, at: base.Add(time.Second), faults: map[string]bool{}},
	}
	beforeAck := nodeResourceFaultsAt(nil, 1, base.Add(2*time.Second), transitions)
	if !beforeAck["absence:source1"] {
		t.Fatalf("faults before collector acknowledgement = %v", beforeAck)
	}
	afterAck := nodeResourceFaultsAt(nil, 3, base.Add(2*time.Second), transitions)
	if afterAck["absence:source1"] {
		t.Fatalf("faults after collector acknowledgement = %v", afterAck)
	}
}
