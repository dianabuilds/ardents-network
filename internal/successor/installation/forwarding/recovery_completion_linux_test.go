package forwarding

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestForwardingRecoveryEnvelopeKeepsImmutablePrefixAndSeparateCompletion(t *testing.T) {
	rows, _, selected, _ := replacementRecordsVector(t)
	whole := encodeReplacementVector(t, rows)
	prefixCount := 54
	prefix := encodeReplacementVector(t, rows[:prefixCount])
	cutRaw := []byte("detached cut byte fixture, not an original-scope proof\n")
	boundary := transitionMessage{Operation: "replacement", Journal: rows[0].Path, Device: rows[0].Device, Inode: rows[0].Inode, PrefixSHA256: digest(prefix), Records: uint32(prefixCount), Generation: selected.binding.Generation, BindingSHA256: digest(selected.raw)}
	cut := transitionCutResult{Boundary: boundary}
	continuation := make([]json.RawMessage, 0, len(rows)-prefixCount)
	for _, row := range rows[prefixCount:] {
		raw, _ := json.Marshal(row)
		continuation = append(continuation, raw)
	}
	result := recoveryCompletion{"ardents-forwarding-recovery-completion-v1", "replacement", boundary.Journal, boundary.Device, boundary.Inode, digest(prefix), uint32(prefixCount), boundary.Generation, boundary.BindingSHA256, digest(cutRaw), "selected-stopped", continuation, digest(whole), ""}
	encode := func(value recoveryCompletion) []byte {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return append(raw, '\n')
	}
	raw := encode(result)
	_, reconstructed, err := decodeRecoveryCompletion(raw, prefix, cutRaw, cut)
	if err != nil || !bytes.Equal(reconstructed, whole) {
		t.Fatal("independent original prefix and continuation lost", err)
	}
	for _, fault := range []string{"operation", "journal", "device", "inode", "prefix", "count", "generation", "binding", "cut", "outcome", "transcript", "sequence", "missing", "null"} {
		bad := result
		bad.Records = append([]json.RawMessage(nil), result.Records...)
		switch fault {
		case "operation":
			bad.Operation = "initial"
		case "journal":
			bad.Journal += "-foreign"
		case "device":
			bad.Device++
		case "inode":
			bad.Inode++
		case "prefix":
			bad.PrefixSHA256 = digest(whole)
		case "count":
			bad.PrefixRecords++
		case "generation":
			bad.Generation = digest([]byte("another generation"))
		case "binding":
			bad.BindingSHA256 = digest([]byte("another binding"))
		case "cut":
			bad.OriginalCutSHA256 = digest([]byte("another cut"))
		case "outcome":
			bad.Outcome = "retired-stopped"
		case "transcript":
			bad.TranscriptSHA256 = digest(prefix)
		case "sequence":
			var row initialRecord
			if json.Unmarshal(bad.Records[0], &row) != nil {
				t.Fatal("bad independent vector")
			}
			row.Sequence--
			bad.Records[0], _ = json.Marshal(row)
		case "missing":
			bad.Records = bad.Records[:len(bad.Records)-1]
		case "null":
			bad.Records = nil
		}
		if _, _, err := decodeRecoveryCompletion(encode(bad), prefix, cutRaw, cut); err == nil {
			t.Fatal("foreign/partial recovery envelope admitted", fault)
		}
	}
	for _, bad := range [][]byte{bytes.TrimSuffix(raw, []byte{'\n'}), append(bytes.Clone(raw), '\n'), bytes.Replace(raw, []byte(`"outcome":`), []byte(`"unknown":true,"outcome":`), 1)} {
		if _, _, err := decodeRecoveryCompletion(bad, prefix, cutRaw, cut); err == nil {
			t.Fatal("ambiguous recovery envelope admitted")
		}
	}
	// Invented inodes, detached records and this codec grant no safe-prefix,
	// original-scope, Sync/Close, manager, Release or recovery permission.
}
