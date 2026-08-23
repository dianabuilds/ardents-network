package endpoint

import (
	"context"
	"net"
	"testing"
)

func TestNameBindingContinuationIsMonotonicAndDestinationExact(t *testing.T) {
	t.Parallel()
	initial := DestinationBinding{
		Name: "alice", Generation: 4, Revision: 8, Authority: "authority-a", Target: [32]byte{1},
		RecordDigest: [32]byte{2}, Commitment: [32]byte{3},
	}
	renewed := initial
	renewed.Revision++
	renewed.RecordDigest[0]++
	renewed.Commitment[0]++
	if !continuesNameBinding(initial, initial) || !continuesNameBinding(initial, renewed) {
		t.Fatal("exact or monotonic same-Target binding did not continue")
	}

	cases := map[string]func(*DestinationBinding){
		"stale revision":   func(value *DestinationBinding) { value.Revision-- },
		"old generation":   func(value *DestinationBinding) { value.Generation-- },
		"different target": func(value *DestinationBinding) { value.Target[0]++ },
		"new authority":    func(value *DestinationBinding) { value.Authority = "authority-b" },
		"different parent": func(value *DestinationBinding) { value.ParentGeneration++ },
		"same version fork": func(value *DestinationBinding) {
			value.RecordDigest[0]++
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := initial
			mutate(&changed)
			if continuesNameBinding(initial, changed) {
				t.Fatalf("invalid binding continued: %+v", changed)
			}
		})
	}
}

func TestNameOriginRecoveryRequiresExactDestinationCommitment(t *testing.T) {
	t.Parallel()
	binding := DestinationBinding{
		Name: "alice", Generation: 1, Revision: 2, Authority: "authority", Target: [32]byte{1},
		RecordDigest: [32]byte{2}, Commitment: [32]byte{3},
	}
	updates := make(chan DestinationBinding)
	request := Request{NameBinding: binding, NameUpdates: updates,
		OpenAttachment: func(context.Context, Recovery) (net.Conn, error) { return nil, nil }}
	if err := validateNameOrigin(request, Credential{Target: binding.Target}); err == nil {
		t.Fatal("recovery accepted a destination commitment unrelated to the resolved Name")
	}
	request.RecoveryBinding.DestinationBinding = binding.Commitment
	if err := validateNameOrigin(request, Credential{Target: binding.Target}); err != nil {
		t.Fatalf("exact Name destination commitment rejected: %v", err)
	}
}
