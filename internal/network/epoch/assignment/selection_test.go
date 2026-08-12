package assignment_test

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
)

func TestSelectIsDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	first, err := assignment.Select([32]byte{1}, 7, [32]byte{2}, "family", []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := assignment.Select([32]byte{1}, 7, [32]byte{2}, "family", []string{"alpha", "beta"})
	if err != nil || first != second {
		t.Fatalf("selection = %q then %q (%v)", first, second, err)
	}
	if _, err := assignment.Select([32]byte{}, 1, [32]byte{}, "family", nil); err == nil {
		t.Fatal("empty domain set accepted")
	}
}
