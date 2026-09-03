//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runSelfTest is the entrypoint for `test-driver self-test`. It runs the
// slice 1 self-test (six synthetic all-valid node events through
// VerifyConvergence) followed by the slice 2 self-test (one successful and
// eight rejecting cases through VerifyAdversaryScenario, see
// runAdversarySelfTest). The
// function exits non-zero on the first failure, including the failing case
// name and observed accept/reason, so a self-test failure is unambiguous.
func runSelfTest(ctx context.Context) error {
	tmp, err := os.MkdirTemp("", "pilot-self-test-")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(tmp)
	fixturesDir := filepath.Join(tmp, "fixtures")
	if err := os.MkdirAll(fixturesDir, 0o700); err != nil {
		return fmt.Errorf("mkdir fixtures: %w", err)
	}
	now := time.Now().UTC()
	fixtures, err := Prebake(fixturesDir, now)
	if err != nil {
		return fmt.Errorf("self-test prebake: %w", err)
	}
	clientPin, sourceAPin, sourceBPin, err := WriteCerts(
		filepath.Join(fixturesDir, "source-ca.pem"),
		filepath.Join(fixturesDir, "source-a.pem"), filepath.Join(fixturesDir, "source-a-key.pem"),
		filepath.Join(fixturesDir, "source-b.pem"), filepath.Join(fixturesDir, "source-b-key.pem"),
		filepath.Join(fixturesDir, "client-ca.pem"),
		filepath.Join(fixturesDir, "client.pem"), filepath.Join(fixturesDir, "client-key.pem"),
		now)
	if err != nil {
		return fmt.Errorf("self-test certs: %w", err)
	}
	if err := WritePlans(fixturesDir, filepath.Join(tmp, "source-a-state"),
		filepath.Join(tmp, "source-b-state"), DefaultSourceAddressA, DefaultSourceAddressB,
		fixtures, clientPin, sourceAPin, sourceBPin, now); err != nil {
		return fmt.Errorf("self-test plans: %w", err)
	}
	nodesDir := filepath.Join(tmp, "nodes")
	if err := os.MkdirAll(nodesDir, 0o700); err != nil {
		return fmt.Errorf("self-test mkdir nodes: %w", err)
	}
	for index := 0; index < 6; index++ {
		raw := fmt.Sprintf(`{"schema":"ardents-source-event-v1","kind":"source-wave-accepted","generation":"%s","epoch":%d,"source_attempts":2,"source_outcomes":["valid","valid","not-attempted","not-attempted"],"latest_completeness":"latest completeness unproven"}`,
			fixtures.Generation, fixtures.EpochNumber)
		if err := os.WriteFile(filepath.Join(nodesDir, fmt.Sprintf("node-%d.json", index+1)),
			[]byte(raw+"\n"), 0o600); err != nil {
			return fmt.Errorf("self-test write node log: %w", err)
		}
	}
	verdict, err := VerifyConvergence(tmp, fixtures.Generation)
	if err != nil {
		return fmt.Errorf("self-test verify: %w", err)
	}
	if !verdict.Accept {
		return fmt.Errorf("self-test: synthetic convergence verdict should accept, got reason %q", verdict.Reason)
	}
	if verdict.DistinctResults != 1 {
		return fmt.Errorf("self-test: synthetic verdict should be 1 distinct set, got %d", verdict.DistinctResults)
	}
	if err := runAdversarySelfTest(ctx); err != nil {
		return err
	}
	fmt.Println("test-driver: self-test passed")
	_ = ctx
	return nil
}

// runAdversarySelfTest exercises VerifyAdversaryScenario against nine
// synthetic nodes/ directories built in isolated temp dirs. The first
// case is the slice-2 happy path (5 honest + 1 probe, all on the
// expected generation) and must accept; the remaining eight cases each
// flip one invariant and must reject with a reason whose substring
// names the offending value. The function exits non-zero on the first
// failure, including the case name and observed accept/reason, so a
// self-test failure is unambiguous.
func runAdversarySelfTest(ctx context.Context) error {
	expected := strings.Repeat("11", 32)
	wrongGen := strings.Repeat("de", 31) + "01"
	honest := [4]string{"valid", "valid", "not-attempted", "not-attempted"}
	probe := [4]string{"valid", "invalid-state", "not-attempted", "not-attempted"}
	fiveHonestOneProbe := func() map[string][4]string {
		m := map[string][4]string{}
		for i := 1; i <= 5; i++ {
			m[fmt.Sprintf("%d", i)] = honest
		}
		m["6"] = probe
		return m
	}
	sixHonest := func() map[string][4]string {
		m := map[string][4]string{}
		for i := 1; i <= 6; i++ {
			m[fmt.Sprintf("%d", i)] = honest
		}
		return m
	}
	cases := []struct {
		name            string
		events          map[string][4]string
		generation      string
		junkNodeID      string
		overlongNodeID  string
		expectAccept    bool
		expectReasonHas string
	}{
		{
			name:         "N1",
			events:       fiveHonestOneProbe(),
			generation:   expected,
			expectAccept: true,
		},
		{
			name:            "N2",
			events:          fiveHonestOneProbe(),
			generation:      wrongGen,
			expectAccept:    false,
			expectReasonHas: "generation",
		},
		{
			name:            "N3",
			events:          sixHonest(),
			generation:      expected,
			expectAccept:    false,
			expectReasonHas: "probe",
		},
		{
			name: "N4",
			events: func() map[string][4]string {
				m := map[string][4]string{}
				for i := 1; i <= 4; i++ {
					m[fmt.Sprintf("%d", i)] = honest
				}
				m["5"] = probe
				m["6"] = probe
				return m
			}(),
			generation:      expected,
			expectAccept:    false,
			expectReasonHas: "probe",
		},
		{
			name: "N5",
			events: func() map[string][4]string {
				m := fiveHonestOneProbe()
				m["6"] = [4]string{"valid", "framing-failed", "not-attempted", "not-attempted"}
				return m
			}(),
			generation:      expected,
			expectAccept:    false,
			expectReasonHas: "outcomes",
		},
		{
			name: "N6",
			events: func() map[string][4]string {
				m := map[string][4]string{}
				for i := 1; i <= 5; i++ {
					if i == 3 {
						m["3"] = probe
					} else {
						m[fmt.Sprintf("%d", i)] = honest
					}
				}
				m["6"] = honest
				return m
			}(),
			generation:      expected,
			expectAccept:    false,
			expectReasonHas: "node-3",
		},
		{
			name: "N7",
			events: func() map[string][4]string {
				m := fiveHonestOneProbe()
				m["3"] = [4]string{"valid", "valid", "valid", "not-attempted"}
				return m
			}(),
			generation:      expected,
			expectAccept:    false,
			expectReasonHas: "outcomes",
		},
		{
			name:            "N8",
			events:          fiveHonestOneProbe(),
			generation:      expected,
			junkNodeID:      "2",
			expectAccept:    false,
			expectReasonHas: "parse",
		},
		{
			name:            "N9",
			events:          fiveHonestOneProbe(),
			generation:      expected,
			overlongNodeID:  "3",
			expectAccept:    false,
			expectReasonHas: "source_outcomes",
		},
	}
	for _, c := range cases {
		tmp, err := os.MkdirTemp("", "pilot-self-test-adversary-")
		if err != nil {
			return fmt.Errorf("self-test-adversary %s: mktemp: %w", c.name, err)
		}
		nodesDir := filepath.Join(tmp, "nodes")
		if err := os.MkdirAll(nodesDir, 0o700); err != nil {
			os.RemoveAll(tmp)
			return fmt.Errorf("self-test-adversary %s: mkdir nodes: %w", c.name, err)
		}
		for nodeID, outcomes := range c.events {
			var raw string
			if c.junkNodeID == nodeID {
				raw = "not json\n"
			} else if c.overlongNodeID == nodeID {
				raw = fmt.Sprintf(
					`{"schema":"ardents-source-event-v1","kind":"source-wave-accepted","generation":%q,"epoch":1,"source_attempts":2,"source_outcomes":[%q,%q,%q,%q,%q],"latest_completeness":"latest completeness unproven"}`,
					c.generation, outcomes[0], outcomes[1], outcomes[2], outcomes[3], "valid")
			} else {
				raw = fmt.Sprintf(
					`{"schema":"ardents-source-event-v1","kind":"source-wave-accepted","generation":%q,"epoch":1,"source_attempts":2,"source_outcomes":[%q,%q,%q,%q],"latest_completeness":"latest completeness unproven"}`,
					c.generation, outcomes[0], outcomes[1], outcomes[2], outcomes[3])
			}
			path := filepath.Join(nodesDir, "node-"+nodeID+".json")
			if err := os.WriteFile(path, []byte(raw+"\n"), 0o600); err != nil {
				os.RemoveAll(tmp)
				return fmt.Errorf("self-test-adversary %s: write node-%s: %w", c.name, nodeID, err)
			}
		}
		verdict, err := VerifyAdversaryScenario(tmp, expected)
		os.RemoveAll(tmp)
		if err != nil {
			return fmt.Errorf("self-test-adversary %s: verify: %w", c.name, err)
		}
		if verdict.Accept != c.expectAccept {
			return fmt.Errorf("self-test-adversary %s: accept=%v, want %v (reason=%q)",
				c.name, verdict.Accept, c.expectAccept, verdict.Reason)
		}
		if !c.expectAccept && c.expectReasonHas != "" && !strings.Contains(verdict.Reason, c.expectReasonHas) {
			return fmt.Errorf("self-test-adversary %s: reason=%q, want substring %q",
				c.name, verdict.Reason, c.expectReasonHas)
		}
		fmt.Printf("test-driver: self-test-adversary %s PASS\n", c.name)
	}
	fmt.Printf("test-driver: self-test-adversary all %d cases passed\n", len(cases))
	_ = ctx
	return nil
}
