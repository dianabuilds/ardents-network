package issuance

import (
	"errors"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
)

func TestFactsRejectDuplicateUnknownAndTrailingInput(t *testing.T) {
	for _, raw := range []string{`{"network":"a","network":"b"}`, `{"unknown":"value"}`, `{} {}`, `[]`, `null`} {
		if _, err := DecodeBatchFacts([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestCompleteBatchFactsRejectDuplicateNetwork(t *testing.T) {
	network := strings.Repeat("01", 32)
	field := `"network":"` + network + `"`
	good := `{` + field +
		`,"issuer":"` + strings.Repeat("02", 32) +
		`","authority":"` + strings.Repeat("03", 32) +
		`","holder":"` + strings.Repeat("04", 32) +
		`","duty_not_before":"2027-01-15T07:00:00Z","duty_not_after":"2027-01-15T09:00:00Z",` +
		`"now":"2027-01-15T08:00:00Z","duty":"13","class":"2","count":"8"}`
	facts, err := DecodeBatchFacts([]byte(good))
	if err != nil || facts.Duty != 13 || facts.Class != 2 || facts.Count != 8 || facts.Network[31] != 1 {
		t.Fatalf("complete declaration lost its facts: %+v, %v", facts, err)
	}
	for _, test := range []struct{ name, value string }{
		{"same value", network},
		{"different value", strings.Repeat("05", 32)},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.Replace(good, field, field+`,"network":"`+test.value+`"`, 1)
			got, err := DecodeBatchFacts([]byte(raw))
			if !errors.Is(err, ErrInvalid) || got != (admission.Facts{}) {
				t.Fatalf("duplicate returned facts or the wrong refusal: %+v, %v", got, err)
			}
		})
	}
}
