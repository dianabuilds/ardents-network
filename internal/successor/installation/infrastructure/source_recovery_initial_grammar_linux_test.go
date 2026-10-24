package infrastructure

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceRecoveryInitialGrammarChecksEveryAvailableRecord(t *testing.T) {
	full, binding, hashes := sourceInitialRecordVector(t, true)
	lines := bytes.SplitAfter(full, []byte{'\n'})
	for count := 1; count <= 115; count++ {
		prefix := bytes.Join(lines[:count], nil)
		err := validateSourceRecoverableInitialPrefix(prefix, binding, hashes, strings.Repeat("9", 64))
		if (count < 41) != (err != nil) {
			t.Fatalf("prefix%d changed complete-account grammar boundary: %v", count, err)
		}
		if count < 115 {
			if _, err := decodeSourceInitialBirthRecords(prefix, binding, hashes, strings.Repeat("9", 64)); err == nil {
				t.Fatal("recovery classifier widened ordinary complete-only reader", count)
			}
		}
		if count < 41 {
			continue
		}
		for position := 0; position < count; position++ {
			var record sourceRecord
			if err := json.Unmarshal(bytes.TrimSuffix(lines[position], []byte{'\n'}), &record); err != nil {
				t.Fatal(err)
			}
			record.Phase += "-foreign"
			line, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			mutated := append([][]byte(nil), lines[:count]...)
			mutated[position] = append(line, '\n')
			if err := validateSourceRecoverableInitialPrefix(bytes.Join(mutated, nil), binding, hashes, strings.Repeat("9", 64)); err == nil {
				t.Fatalf("prefix%d ignored a foreign phase at sequence%d", count, position)
			}
		}
	}
}

func TestSourceRecoveryInitialGrammarRefusesForeignHistoryAndLegacyBirth(t *testing.T) {
	full, binding, hashes := sourceInitialRecordVector(t, true)
	lines := bytes.SplitAfter(full, []byte{'\n'})
	for _, count := range []int{41, 60, 104, 114, 115} {
		prefix := bytes.Join(lines[:count], nil)
		if err := validateSourceRecoverableInitialPrefix(prefix, binding, hashes, strings.Repeat("8", 64)); err == nil {
			t.Fatal("foreign original history commitment accepted", count)
		}
		if err := validateSourceRecoverableInitialPrefix(prefix[:len(prefix)-1], binding, hashes, strings.Repeat("9", 64)); err == nil {
			t.Fatal("torn prefix accepted", count)
		}
	}
	legacy, binding, hashes := sourceInitialRecordVector(t, false)
	if err := validateSourceRecoverableInitialPrefix(legacy, binding, hashes, strings.Repeat("9", 64)); err == nil {
		t.Fatal("legacy complete-only journal invented a pending birth")
	}
}
