package processfixture

import (
	"errors"
	"testing"
)

func TestAdmissionPresentationsRequiresDistinctSuccessfulOriginalJOINs(t *testing.T) {
	refused := errors.New("Receiving declined JOIN")
	for _, scenario := range []struct {
		name    string
		binding [32]byte
		token   [32]byte
		outcome error
		want    bool
	}{
		{"fresh", [32]byte{2}, [32]byte{3}, nil, true},
		{"repeat across duties", [32]byte{2}, [32]byte{9}, nil, false},
		{"repeat binding", [32]byte{1}, [32]byte{3}, nil, false},
		{"missing binding", [32]byte{}, [32]byte{3}, nil, false},
		{"refused JOIN", [32]byte{2}, [32]byte{3}, refused, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var evidence AdmissionPresentations
			evidence.Observe(false, [32]byte{}, [32]byte{9}, nil)
			evidence.Observe(true, [32]byte{1}, [32]byte{1}, nil)
			evidence.Observe(true, scenario.binding, scenario.token, scenario.outcome)
			if err := evidence.Check(2); (err == nil) != scenario.want {
				t.Fatalf("evidence check = %v, want success=%t", err, scenario.want)
			}
		})
	}
}
