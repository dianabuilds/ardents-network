package issuance

import "testing"

func TestFactsRejectDuplicateUnknownAndTrailingInput(t *testing.T) {
	for _, raw := range []string{`{"network":"a","network":"b"}`, `{"unknown":"value"}`, `{} {}`, `[]`, `null`} {
		if _, err := DecodeBatchFacts([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
