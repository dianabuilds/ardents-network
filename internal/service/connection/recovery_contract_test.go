package connection

import "testing"

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
