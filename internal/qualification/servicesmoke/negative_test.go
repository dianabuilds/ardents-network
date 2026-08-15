package servicesmoke

import "testing"

func TestStage3NegativeReceiptIncludesRecoveryQueueBound(t *testing.T) {
	value := negativeReceipt{Schema: "ardents-h3-service-negative-v1", Negatives: map[string]bool{},
		Mechanisms: map[string]string{}, Operations: map[string]bool{
			"backpressure": true, "cancellation": true, "partial-write": true, "recovery-queue-full": true,
		}}
	for index := range 24 {
		value.Negatives[string(rune('a'+index))] = true
		value.Mechanisms[string(rune('a'+index))] = "test"
	}
	if !validNegativeReceiptShape(value) {
		t.Fatal("integrated Stage 3/4 negative receipt was rejected")
	}
	delete(value.Operations, "recovery-queue-full")
	if validNegativeReceiptShape(value) {
		t.Fatal("receipt without recovery queue negative was accepted")
	}
}
