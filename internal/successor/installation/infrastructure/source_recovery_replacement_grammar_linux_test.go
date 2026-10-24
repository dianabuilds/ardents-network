package infrastructure

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Independent closed-record vector only. No supplied identities, phase or
// parsed prefix provide native birth, original-cut proof or Source admission.
func TestSourceRecoveryReplacementEveryCanonicalPrefix(t *testing.T) {
	raw, binding, hashes := sourceReplacementRecordVector(t)
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	admitted, previous := strings.Repeat("3", 64), strings.Repeat("4", 64)
	supported := map[int]bool{}
	for _, count := range []int{11, 13, 14, 16, 17, 19, 20, 22, 23, 25, 26, 28, 29, 31, 32, 34, 35, 37, 38, 40, 41, 43, 44, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55} {
		supported[count] = true
	}
	if sourceReplacementRecoveryPrefixSupported(nil) {
		t.Fatal("empty replacement prefix supported")
	}
	for n := 1; n <= 55; n++ {
		prefix := append(bytes.Join(lines[:n], []byte{'\n'}), '\n')
		records, err := decodeSourceRecoveryPrefix(prefix, 55)
		if err != nil || sourceReplacementRecoveryPrefixSupported(records) != supported[n] {
			t.Fatalf("replacement continuation count%d differs from independent oracle: %v", n, err)
		}
		if err := validateSourceReplacementPrefix(prefix, binding, hashes, admitted, previous); err != nil {
			t.Fatalf("canonical prefix%d: %v", n, err)
		}
		if n < 55 {
			if _, err := decodeSourceReplacementRecords(prefix, binding, hashes, admitted, previous); err == nil {
				t.Fatal("ordinary complete reader accepted canonical partial prefix", n)
			}
		}
		for i := range n {
			var record sourceRecord
			if err := json.Unmarshal(lines[i], &record); err != nil {
				t.Fatal(err)
			}
			record.Phase = "foreign-phase"
			changed, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			copyLines := append([][]byte(nil), lines[:n]...)
			copyLines[i] = changed
			foreign := append(bytes.Join(copyLines, []byte{'\n'}), '\n')
			if err := validateSourceReplacementPrefix(foreign, binding, hashes, admitted, previous); err == nil {
				t.Fatal("foreign phase at recorded prefix admitted", n, i)
			}
		}
	}
}

func TestSourceRecoveryReplacementPublicationPreservesOriginalInode(t *testing.T) {
	raw, binding, hashes := sourceReplacementRecordVector(t)
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	for _, at := range []int{50, 52} {
		var record sourceRecord
		if err := json.Unmarshal(lines[at], &record); err != nil {
			t.Fatal(err)
		}
		record.Inode++
		changed, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		copyLines := append([][]byte(nil), lines[:at+1]...)
		copyLines[at] = changed
		foreign := append(bytes.Join(copyLines, []byte{'\n'}), '\n')
		if err := validateSourceReplacementPrefix(foreign, binding, hashes, strings.Repeat("3", 64), strings.Repeat("4", 64)); err == nil {
			t.Fatal("written publication adopted foreign inode", at)
		}
	}
}
