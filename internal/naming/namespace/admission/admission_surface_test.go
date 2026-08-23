package admission

import (
	"crypto/sha256"
	"testing"
)

func TestAdmissionIssueRejectsFullSurfaceWithoutBlockingAnotherSurface(t *testing.T) {
	gate, err := NewAdmission([32]byte{1}, [32]byte{2}, 3, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	resolution, ok := gate.surfaces["resolution"]
	if !ok {
		t.Fatal("resolution surface is absent")
	}
	resolution.mu.Lock()
	for index := range 4096 {
		var digest [32]byte
		digest[0] = byte(index)
		digest[1] = byte(index >> 8)
		resolution.spent[digest] = 1_000
	}
	resolution.nextExpiry = 1_000
	resolution.mu.Unlock()

	operation := sha256.Sum256([]byte("operation"))
	isolation := sha256.Sum256([]byte("isolation"))
	if _, err := gate.Issue(900, "resolution", operation, isolation, 1_000, [16]byte{1}); err == nil {
		t.Fatal("full resolution surface issued a challenge")
	}
	if _, err := gate.Issue(900, "renewal-update", operation, isolation, 1_000, [16]byte{2}); err != nil {
		t.Fatalf("independent renewal surface issue: %v", err)
	}
}
