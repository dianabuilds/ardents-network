package network

import (
	"encoding/hex"
	"testing"
)

func TestRoleAssignmentFrozenTranscript(t *testing.T) {
	// This digest is fixed independently of the production encoder.
	want := "81ac344fc68b5ace8b27ba366012f778ae680e0315fa1f677080367f5e287255"
	got := RoleAssignmentDigest([32]byte{1}, 7, [32]byte{2}, "family", "alpha")
	if hex.EncodeToString(got[:]) != want {
		t.Fatalf("assignment transcript changed: %x", got)
	}
	first, err := AssignRoleDomain([32]byte{1}, 7, [32]byte{2}, "family", []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := AssignRoleDomain([32]byte{1}, 7, [32]byte{2}, "family", []string{"beta", "alpha"})
	if err != nil || reordered != first {
		t.Fatalf("order changed assignment: %q / %q (%v)", first, reordered, err)
	}
	if _, err := AssignRoleDomain([32]byte{1}, 7, [32]byte{2}, "family", []string{first, first}); err == nil {
		t.Fatal("selected digest tie admitted")
	}
	if _, err := AssignRoleDomain([32]byte{1}, 7, [32]byte{2}, "family", nil); err == nil {
		t.Fatal("empty assignment admitted")
	}
}
