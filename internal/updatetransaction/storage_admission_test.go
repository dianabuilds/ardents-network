package updatetransaction

import "testing"

// TestResourceEnvelopeExactBoundary uses a worked literal: with an eight-byte
// allocation unit, 3-byte artifact + 4-byte manifest + nine 139-byte journal
// records + 2-byte current record require 8 + 8 + 1,256 + 8 = 1,280 bytes.
func TestResourceEnvelopeExactBoundary(t *testing.T) {
	artifact := []byte("abc")
	manifest := []byte("wxyz")
	current := []byte("ok")
	accepted := resourceObservation{allocationUnit: 8, availableBytes: 1280, availableItems: 15, itemsKnown: true}
	if err := requireResourceEnvelope(accepted, artifact, manifest, current); err != nil {
		t.Fatalf("exact envelope rejected: %v", err)
	}
	accepted.availableBytes = 1279
	if err := requireResourceEnvelope(accepted, artifact, manifest, current); err == nil {
		t.Fatal("first byte below envelope was accepted")
	}
	accepted.availableBytes = 1280
	accepted.availableItems = 14
	if err := requireResourceEnvelope(accepted, artifact, manifest, current); err == nil {
		t.Fatal("first object below envelope was accepted")
	}
}

// TestObserveOwnedStorage observes the native development surface through its
// private implementation boundary. It makes no quota or power-loss claim.
func TestObserveOwnedStorage(t *testing.T) {
	root, _ := applyCheckpointRequest(t)
	observation, err := observeOwnedStorage(root)
	if err != nil {
		t.Fatalf("observe owned storage: %v", err)
	}
	if observation.allocationUnit == 0 || observation.availableBytes == 0 {
		t.Fatalf("invalid storage observation: %+v", observation)
	}
}
