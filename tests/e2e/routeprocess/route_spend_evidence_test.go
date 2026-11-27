package routeprocess_test

import (
	"bytes"
	"errors"
	"testing"
)

func TestRouteSpendEvidenceRequiresOneCompleteCommittedRecord(t *testing.T) {
	before := []byte("original-history")
	pending := append(bytes.Clone(before), make([]byte, 41)...)
	committed := bytes.Clone(pending)
	committed[len(committed)-1] = 1
	invalid := bytes.Clone(pending)
	invalid[len(invalid)-1] = 2
	for _, tc := range []struct {
		name      string
		observed  []byte
		committed bool
		invalid   bool
	}{
		{name: "unchanged", observed: before},
		{name: "partial append", observed: pending[:len(pending)-1]},
		{name: "complete body without marker", observed: pending},
		{name: "one committed spend", observed: committed, committed: true},
		{name: "invalid marker", observed: invalid, invalid: true},
		{name: "second append", observed: append(bytes.Clone(committed), 0), invalid: true},
		{name: "original bytes replaced", observed: []byte("changed-history"), invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := committedRouteSpend(before, tc.observed)
			if got != tc.committed || (err != nil) != tc.invalid {
				t.Fatalf("committed = %t, error = %v, want committed = %t and invalid = %t", got, err, tc.committed, tc.invalid)
			}
		})
	}
}

// committedRouteSpend observes exactly one appended receiving record. A writer
// can expose the record's bytes before it writes the final commit marker.
func committedRouteSpend(before, observed []byte) (bool, error) {
	const recordBytes = 32 + 8 + 1
	if !bytes.HasPrefix(observed, before) || len(observed) > len(before)+recordBytes {
		return false, errors.New("receiving history changed before the original spend or appended multiple spends")
	}
	if len(observed) < len(before)+recordBytes {
		return false, nil
	}
	switch observed[len(observed)-1] {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, errors.New("receiving spend has an invalid commit marker")
	}
}
