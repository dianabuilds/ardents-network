package state

import "testing"

func TestSelectIsDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	first, err := selectEpochDomain([32]byte{1}, 7, [32]byte{2}, "family", []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := selectEpochDomain([32]byte{1}, 7, [32]byte{2}, "family", []string{"alpha", "beta"})
	if err != nil || first != second {
		t.Fatalf("selection = %q then %q (%v)", first, second, err)
	}
	if _, err := selectEpochDomain([32]byte{}, 1, [32]byte{}, "family", nil); err == nil {
		t.Fatal("empty domain set accepted")
	}
}
