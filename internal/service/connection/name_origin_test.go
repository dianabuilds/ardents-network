package connection

import "testing"

func TestNameOriginPinsTargetAndContinuity(t *testing.T) {
	t.Parallel()
	binding := DestinationBinding{Name: "service", Generation: 1, Revision: 2, Authority: "authority",
		Target: [32]byte{1}, ParentName: "parent", ParentGeneration: 3, RecordDigest: [32]byte{4}, Commitment: [32]byte{5}}
	updates := make(chan DestinationBinding)
	if err := ValidateNameOrigin(binding, updates, binding.Target, true, binding.Commitment); err != nil {
		t.Fatal(err)
	}
	if err := ValidateNameOrigin(binding, updates, binding.Target, true, [32]byte{9}); err == nil {
		t.Fatal("recovery accepted a different destination commitment")
	}
	continued := binding
	continued.Revision++
	continued.RecordDigest = [32]byte{8}
	if !ContinuesNameOrigin(binding, continued) {
		t.Fatal("higher same-target revision was rejected")
	}
	continued.Target[0]++
	if ContinuesNameOrigin(binding, continued) {
		t.Fatal("target substitution was accepted")
	}
}

func TestValidateRecoveryRequiresAnExactFiniteContract(t *testing.T) {
	t.Parallel()
	recovery := Recovery{CandidateView: [32]byte{1}, IsolationContext: [32]byte{2},
		DestinationBinding: [32]byte{3}, RouteProfile: Profile,
		WorkSafetyNotAfter: 20, WorkSafetyMaximum: 30, NoNewRecoveryAfter: 15}
	if err := ValidateRecovery(true, recovery, 10, 30); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRecovery(false, recovery, 10, 30); err == nil {
		t.Fatal("recovery contract was accepted without an Attachment opener")
	}
	recovery.NoNewRecoveryAfter = 21
	if err := ValidateRecovery(true, recovery, 10, 30); err == nil {
		t.Fatal("recovery after Work Safety was accepted")
	}
}
