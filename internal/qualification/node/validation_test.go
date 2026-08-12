package node

import (
	"bytes"
	"os"
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
	raw, err := ReadCandidateBuildIdentity([]string{binary})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"go_version":"go1.26.5"`)) || !bytes.Contains(raw, []byte(`"dependencies"`)) {
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
