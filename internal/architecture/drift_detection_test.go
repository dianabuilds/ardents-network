package architecture

import (
	"regexp"
	"strings"
	"testing"
)

// Security-critical installation mechanisms exist as deliberate forwarding and
// source copies that must stay in sync (triage C1; audit X-4/X-5). This gate
// turns silent divergence into a failure: each declared pair is compared after
// erasing the variant role vocabulary, and its normalized line similarity must
// stay at or above the recorded baseline. Re-sync both sides when editing one;
// lowering a baseline is a deliberate decision, not a routine fix. Consolidating
// a pair into one owner remains a separate architecture change (C1 step-2).

type driftPair struct {
	name      string
	left      string
	right     string
	normalize [][2]string
	minimum   float64
}

var roleVocabulary = [][2]string{{"forwarding", ""}, {"source", ""}}

var phaseVocabulary = [][2]string{{"start", ""}, {"retirement", ""}, {"stop", ""}}

const (
	forwardingDir     = "internal/successor/installation/forwarding/"
	infrastructureDir = "internal/successor/installation/infrastructure/"
	systemdDir        = "internal/successor/installation/systemd/"
)

var driftPairs = []driftPair{
	{"systemd control", systemdDir + "forwarding_control_linux.go", systemdDir + "source_control_linux.go", roleVocabulary, 0.97},
	{"transition scope", forwardingDir + "transition_scope_linux.go", infrastructureDir + "source_transition_scope_linux.go", roleVocabulary, 0.96},
	{"principal", forwardingDir + "principal_linux.go", infrastructureDir + "source_principal_linux.go", roleVocabulary, 0.82},
	{"transition process", forwardingDir + "transition_process_linux.go", infrastructureDir + "source_transition_process_linux.go", roleVocabulary, 0.95},
	{"runtime identity", forwardingDir + "runtime_identity_linux.go", infrastructureDir + "source_runtime_identity_linux.go", roleVocabulary, 0.92},
	{"account", forwardingDir + "account_linux.go", infrastructureDir + "source_account_linux.go", roleVocabulary, 0.76},
	{"start barrier", forwardingDir + "start_barrier_linux.go", infrastructureDir + "source_start_barrier_linux.go", roleVocabulary, 0.87},
	{"start/retirement storage", forwardingDir + "start_storage_linux.go", forwardingDir + "retirement_storage_linux.go", phaseVocabulary, 0.83},
}

var (
	driftBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	driftLineComment  = regexp.MustCompile(`//.*$`)
	driftWhitespace   = regexp.MustCompile(`\s+`)
)

func TestDeclaredTwinPairsDoNotDrift(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, pair := range driftPairs {
		pair := pair
		t.Run(pair.name, func(t *testing.T) {
			t.Parallel()
			left := driftLineCounts(readProjectFile(t, root, pair.left), pair.normalize)
			right := driftLineCounts(readProjectFile(t, root, pair.right), pair.normalize)
			score := driftDice(left, right)
			if score < pair.minimum {
				t.Errorf("twin pair %q drifted: similarity %.3f below baseline %.3f\n  %s\n  %s\nRe-sync both sides, or record a deliberate baseline change.",
					pair.name, score, pair.minimum, pair.left, pair.right)
			}
		})
	}
}

func driftLineCounts(data []byte, normalize [][2]string) map[string]int {
	text := driftBlockComment.ReplaceAllString(string(data), "")
	counts := make(map[string]int)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(driftLineComment.ReplaceAllString(line, ""))
		if line == "" {
			continue
		}
		line = strings.ToLower(line)
		for _, sub := range normalize {
			line = strings.ReplaceAll(line, sub[0], sub[1])
		}
		line = driftWhitespace.ReplaceAllString(line, " ")
		counts[line]++
	}
	return counts
}

func driftDice(left, right map[string]int) float64 {
	intersection, total := 0, 0
	for line, count := range left {
		total += count
		if other := right[line]; other < count {
			intersection += other
		} else {
			intersection += count
		}
	}
	for _, count := range right {
		total += count
	}
	if total == 0 {
		return 1
	}
	return 2 * float64(intersection) / float64(total)
}
