package fixedfile

import "testing"

func TestReplacementPrefixPreservesClosedCopyImages(t *testing.T) {
	previous, candidate := []byte("old-complete"), []byte("new-complete")
	for _, current := range []string{"", "o", "old-", "old-complete", "n", "new-", "new-complete"} {
		if !ReplacementPrefixAllowed([]byte(current), previous, candidate) {
			t.Fatal("authorized prefix refused", current)
		}
	}
	for _, current := range []string{"old-complete!", "new-complete!", "old-cnew", "new-cold", "foreign", "x"} {
		if ReplacementPrefixAllowed([]byte(current), previous, candidate) {
			t.Fatal("foreign image accepted", current)
		}
	}
	if ReplacementPrefixAllowed(nil, nil, candidate) || ReplacementPrefixAllowed(nil, previous, nil) {
		t.Fatal("missing complete image accepted")
	}
}
