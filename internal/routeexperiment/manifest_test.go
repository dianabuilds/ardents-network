package routeexperiment

import "testing"

func TestFixedWorkloadScheduleIsCompleteAndDeterministic(t *testing.T) {
	t.Parallel()
	first := fixedWorkloadSchedule([]byte("fixed master seed"))
	second := fixedWorkloadSchedule([]byte("fixed master seed"))
	for _, condition := range []string{"direct", "c3", "c5-c2"} {
		if len(first[condition]) != 26 || first[condition][0] != second[condition][0] {
			t.Fatalf("condition %s schedule is incomplete or unstable", condition)
		}
		seen := make(map[string]bool, 26)
		for _, workload := range first[condition] {
			if seen[workload.Seed] || len(workload.Seed) != 64 {
				t.Fatalf("condition %s contains a duplicate or invalid seed", condition)
			}
			seen[workload.Seed] = true
		}
	}
}
