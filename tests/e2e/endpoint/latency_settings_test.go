package endpoint

import (
	"strings"
	"testing"
)

func TestLatencyFixtureRendersDeclaredParticipantSettings(t *testing.T) {
	t.Setenv("ARDENTS_LATENCY_COLD", "")
	t.Setenv("ARDENTS_LATENCY_WARM", "")
	t.Setenv("ARDENTS_LATENCY_SEED", "")
	cold, warm, seed := LatencyDriverCounts(t)
	if cold != 3 || warm != 3 || seed != "seed-1" {
		t.Fatalf("short-run defaults = %d/%d/%q", cold, warm, seed)
	}
	t.Setenv("ARDENTS_LATENCY_COLD", "1")
	t.Setenv("ARDENTS_LATENCY_WARM", "1000")
	t.Setenv("ARDENTS_LATENCY_SEED", "seed-3")
	cold, warm, seed = LatencyDriverCounts(t)
	unit := RenderLatencyUnit("/participant", "/original", "/control", "/stand", cold, warm, seed)
	for _, line := range []string{
		"ExecStart=/participant -test.run=^TestNativeEndpointColdWarmLatencyParticipant$ -test.v -test.timeout=28m",
		"Environment=ARDENTS_LATENCY_ROOT=/original",
		"Environment=ARDENTS_LATENCY_CONTROL=/control",
		"Environment=ARDENTS_LATENCY_COLD=1",
		"Environment=ARDENTS_LATENCY_WARM=1000",
		"Environment=ARDENTS_LATENCY_SEED=seed-3",
		"ReadWritePaths=/stand /etc/ardents",
	} {
		if !strings.Contains(unit, "\n"+line+"\n") {
			t.Errorf("qualification unit misses %q", line)
		}
	}
}
