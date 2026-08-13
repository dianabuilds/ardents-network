package node

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestCampaignModesHaveFrozenDurations(t *testing.T) {
	cases := map[string]time.Duration{
		"short":          0,
		"churn-2h":       2 * time.Hour,
		"unattended-24h": 24 * time.Hour,
	}
	for mode, want := range cases {
		input := Campaign{FixtureRoot: "fixture", EvidenceRoot: "evidence", ComposeFile: "compose", Mode: mode}
		if err := validateNodeSpecialInput(input, mode); err != nil {
			t.Errorf("mode %q rejected: %v", mode, err)
		}
		if got := campaignDuration(mode); got != want {
			t.Errorf("duration for %q = %s, want %s", mode, got, want)
		}
	}
	if got := campaignDuration("combined"); got >= 0 {
		t.Fatalf("unknown duration = %s, want negative", got)
	}
}

func TestCandidateBuildIdentityReadsBinaryInsteadOfSelfReport(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := readCandidateBuildIdentity([]string{binary})
	if err != nil {
		t.Fatal(err)
	}
	version := []byte(`"go_version":"` + runtime.Version() + `"`)
	if !bytes.Contains(raw, version) || !bytes.Contains(raw, []byte(`"dependencies"`)) {
		t.Fatalf("build identity is incomplete: %s", raw)
	}
}

func TestCandidateInspectTreatsPIDZeroAsObservedExit(t *testing.T) {
	if _, found, err := nodeCandidateFromInspect("node1", "abc123def456", "abc123def456789\t0\n"); err != nil || found {
		t.Fatalf("PID zero = found %v, err %v; want observed exit", found, err)
	}
	candidate, found, err := nodeCandidateFromInspect("node1", "abc123def456", "abc123def456789\t42\n")
	if err != nil || !found || candidate.PID != 42 || candidate.Service != "node1" {
		t.Fatalf("running candidate = %+v, found %v, err %v", candidate, found, err)
	}
	if _, _, err := nodeCandidateFromInspect("node1", "abc123def456", "abc123def456789\tnot-a-pid\n"); err == nil {
		t.Fatal("malformed host PID was accepted")
	}
}

func TestNofileInjectionAcceptsNoCandidateControlledInputs(t *testing.T) {
	input := Campaign{Mode: "inject", Injection: "nofile"}
	if err := validateNodeInjectionInput(input); err != nil {
		t.Fatalf("nofile injection rejected: %v", err)
	}
	input.Addresses = []string{"127.0.0.1:1"}
	if err := validateNodeInjectionInput(input); err == nil {
		t.Fatal("nofile injection accepted an irrelevant address")
	}
}

func TestMachineResultRejectsFailedOrMalformedStimulus(t *testing.T) {
	expected := "EMFILE descriptor occupancy completed"
	if err := classifyNodeMachineResult([]byte(`{"verdict":"pass","reason":"EMFILE descriptor occupancy completed"}`),
		0, expected); err != nil {
		t.Fatalf("exact machine result rejected: %v", err)
	}
	for _, raw := range [][]byte{
		[]byte(`{"verdict":"fail","reason":"EMFILE injector could not establish a descriptor stimulus"}`),
		[]byte(`{"verdict":"pass","reason":"different"}`),
		[]byte("docker daemon unavailable"),
	} {
		if err := classifyNodeMachineResult(raw, 0, expected); err == nil {
			t.Fatalf("invalid machine result accepted: %s", raw)
		}
	}
}

func TestMachineCommandSeparatesCandidateFailureFromInvalidHarness(t *testing.T) {
	failure := []byte(`{"verdict":"fail","reason":"candidate rejected probe"}`)
	if err := classifyNodeMachineResult(failure, 1, "pass reason"); err == nil || errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("candidate failure = %v, want ordinary fail", err)
	}
	for _, test := range []struct {
		raw  []byte
		exit int
	}{
		{failure, 2},
		{[]byte(`{"verdict":"pass","reason":"pass reason"}`), 1},
		{[]byte("daemon unavailable"), 1},
	} {
		if err := classifyNodeMachineResult(test.raw, test.exit, "pass reason"); !errors.Is(err, errInvalidNodeCampaign) {
			t.Fatalf("machine result %s/exit %d = %v, want invalid", test.raw, test.exit, err)
		}
	}
}

func TestProductCommandSeparatesCandidateOutputFromDockerFailure(t *testing.T) {
	dockerErr := invalidNodeCampaign(errors.New("docker daemon unavailable"))
	if err := nodeProductCommandError(dockerErr, "refresh network state:", "candidate failed"); !errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("Docker failure = %v, want invalid", err)
	}
	productErr := invalidNodeCampaign(errors.New("refresh network state: rejected"))
	if err := nodeProductCommandError(productErr, "refresh network state:", "candidate failed"); err == nil || errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("product failure = %v, want ordinary candidate failure", err)
	}
}

func TestReadyEventRequiresExactState(t *testing.T) {
	if !nodeReadyEvent([]byte("{\"kind\":\"lifecycle\",\"state\":\"READY\"}\n")) {
		t.Fatal("READY event rejected")
	}
	for _, raw := range [][]byte{
		[]byte(`{"state":"NOT_READY"}`),
		[]byte(`{"message":"candidate emitted \\"state\\":\\"READY\\""}`),
		[]byte(`not-json {"state":"READY"}`),
		[]byte(`node1  | {"state":"READY"}`),
	} {
		if nodeReadyEvent(raw) {
			t.Fatalf("non-event READY accepted: %s", raw)
		}
	}
}

func TestDiskFullStimulusMarkerRequiresExactLine(t *testing.T) {
	if !nodeLogContainsExactLine([]byte("before\n"+nodeDiskFullStimulus+"\nafter\n"), nodeDiskFullStimulus) {
		t.Fatal("exact disk-full stimulus marker rejected")
	}
	if nodeLogContainsExactLine([]byte("prefix "+nodeDiskFullStimulus+" suffix\n"), nodeDiskFullStimulus) {
		t.Fatal("embedded disk-full stimulus marker accepted")
	}
}
