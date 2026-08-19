package blockedverify

import "testing"

func TestIndependentG4ReceiptRequiresPhaseSpecificObservation(t *testing.T) {
	receipt := []byte(`{"schema":"ardents-h3-g4-receipt-v1","phase":"after-useful-work-prefix","checkpoint":{"regime":true,"attempt":true,"contacts":1},"reopened":{"kind":"g4-reopen","phase":"after-useful-work-prefix","terminal":"bridge-interrupted","attempt":true,"contacts":1},"observation":"publisher-progress"}`)
	if !validFinalG4Receipt(receipt, "after-useful-work-prefix") {
		t.Fatal("useful-work receipt was rejected")
	}
	receipt = []byte(`{"schema":"ardents-h3-g4-receipt-v1","phase":"after-useful-work-prefix","checkpoint":{"regime":true,"attempt":true,"contacts":1},"reopened":{"kind":"g4-reopen","phase":"after-useful-work-prefix","terminal":"bridge-interrupted","attempt":true,"contacts":1},"observation":"candidate-state-root"}`)
	if validFinalG4Receipt(receipt, "after-useful-work-prefix") {
		t.Fatal("adapter-start evidence was relabeled as useful work")
	}
}
