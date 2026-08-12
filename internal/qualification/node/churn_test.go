package node

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestQuiescenceNamesMissingService(t *testing.T) {
	baseline := quiescenceSamples()
	after := baseline[:len(baseline)-1]
	err := validateNodeQuiescentResources(baseline, after)
	if err == nil || !strings.Contains(err.Error(), "node2") || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v, want missing node2", err)
	}
}

func TestQuiescenceAcceptsBoundedGrowth(t *testing.T) {
	baseline := quiescenceSamples()
	after := quiescenceSamples()
	for index := range after {
		after[index].FDs += 1
		after[index].PIDs += 2
		after[index].Raw["memory.stat"] = "anon 5242880\nsock 0\nslab 1048576\n"
	}
	if err := validateNodeQuiescentResources(baseline, after); err != nil {
		t.Fatal(err)
	}
}

func quiescenceSamples() []nodeResourceSnapshot {
	result := make([]nodeResourceSnapshot, 0, 5)
	for _, service := range []string{"source1", "source2", "endpoint", "node1", "node2"} {
		result = append(result, nodeResourceSnapshot{Service: service, FDs: 8, Sockets: 1, PIDs: 8,
			Raw: map[string]string{"memory.stat": "anon 1048576\nsock 0\nslab 524288\n"}})
	}
	return result
}

func TestQuiescenceVerdictSeparatesCandidateAndEvidenceFailures(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{nil, "pass"},
		{errors.New("candidate retained resources"), "fail"},
		{invalidNodeCampaign(errors.New("observer unavailable")), "invalid"},
		{context.Canceled, "invalid"},
		{context.DeadlineExceeded, "invalid"},
	} {
		if got := nodeQuiescenceVerdict(test.err); got != test.want {
			t.Fatalf("verdict for %v = %q, want %q", test.err, got, test.want)
		}
	}
}

func TestQuiescenceRejectsMissingMemoryCounterAsInvalid(t *testing.T) {
	baseline, after := quiescenceSamples(), quiescenceSamples()
	after[0].Raw["memory.stat"] = "anon 1048576\nsock 0\n"
	err := validateNodeQuiescentResources(baseline, after)
	if !errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("missing counter error = %v, want invalid campaign", err)
	}
}

func TestQuiescenceRejectsEndpointGrowth(t *testing.T) {
	baseline, after := quiescenceSamples(), quiescenceSamples()
	after[2].FDs += 17
	err := validateNodeQuiescentResources(baseline, after)
	if err == nil || errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("endpoint resource error = %v, want candidate failure", err)
	}
}
