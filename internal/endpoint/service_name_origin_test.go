package endpoint

import (
	"context"
	"net"
	"testing"
)

func TestNameBindingContinuationIsMonotonicAndDestinationExact(t *testing.T) {
	t.Parallel()
	initial := destinationBinding{
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

	cases := map[string]func(*destinationBinding){
		"stale revision":   func(value *destinationBinding) { value.Revision-- },
		"old generation":   func(value *destinationBinding) { value.Generation-- },
		"different target": func(value *destinationBinding) { value.Target[0]++ },
		"new authority":    func(value *destinationBinding) { value.Authority = "authority-b" },
		"different parent": func(value *destinationBinding) { value.ParentGeneration++ },
		"same version fork": func(value *destinationBinding) {
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
	binding := destinationBinding{
		Name: "alice", Generation: 1, Revision: 2, Authority: "authority", Target: [32]byte{1},
		RecordDigest: [32]byte{2}, Commitment: [32]byte{3},
	}
	updates := make(chan destinationBinding)
	request := connectionInput{NameBinding: binding, NameUpdates: updates,
		OpenAttachment: func(context.Context, routeRecovery) (net.Conn, error) { return nil, nil }}
	if err := validateNameOrigin(request, publicationCredential{Target: binding.Target}); err == nil {
		t.Fatal("recovery accepted a destination commitment unrelated to the resolved Name")
	}
	request.RecoveryBinding.DestinationBinding = binding.Commitment
	if err := validateNameOrigin(request, publicationCredential{Target: binding.Target}); err != nil {
		t.Fatalf("exact Name destination commitment rejected: %v", err)
	}
}
