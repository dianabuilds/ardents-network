package infrastructure

import (
	"bytes"
	"encoding/json"
	"strings"
	"syscall"
	"testing"
)

// Reuse the independently encoded closed replacement byte oracle. Its numeric
// native identities isolate grammar rules; this is no physical or Release proof.
func TestSourceRecoveryRecordsKeepCompleteReplacementGrammar(t *testing.T) {
	full, binding, hashes := sourceReplacementRecordVector(t)
	selected := sourceReplacementBindingVector()
	records, err := decodeSourceRecoveryPrefix(full, 55)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.SplitAfter(full, []byte{'\n'})
	for cut := 1; cut <= 55; cut++ {
		prefix := bytes.Join(lines[:cut], nil)
		remainder := append(make(sourceRecoveryRecords, 0), records[cut:]...)
		completion := sourceRecoveryCompletion{Schema: "ardents-source-recovery-completion-v1", Operation: "replacement",
			Journal: records[0].Path, Device: records[0].Device, Inode: records[0].Inode,
			PrefixSHA256: digest(prefix), PrefixRecords: cut, Generation: binding.Configuration.Generation,
			BindingSHA256: digest(selected), Records: remainder, TranscriptSHA256: digest(full)}
		info := sourceRecoveryCodecInfo{size: int64(len(prefix)), mode: 0600,
			stat: syscall.Stat_t{Dev: records[0].Device, Ino: records[0].Inode, Nlink: 1}}
		transcript, err := decodeSourceRecoveryCompletion(marshalSourceRecoveryCodec(t, completion), prefix, info,
			completion.Journal, completion.Operation, completion.Generation, completion.BindingSHA256)
		if err != nil || !bytes.Equal(transcript, full) {
			t.Fatalf("prefix%d changed full canonical oracle: %v", cut, err)
		}
		owned, err := decodeSourceRecoveredRecords(transcript, selected, "replacement", hashes, strings.Repeat("3", 64), strings.Repeat("4", 64))
		if err != nil || len(owned) == 0 {
			t.Fatalf("prefix%d did not preserve complete replacement grammar: %v", cut, err)
		}
		if _, err := decodeSourceRecoveredRecords(prefix[:len(prefix)-1], selected, "replacement", hashes, strings.Repeat("3", 64), strings.Repeat("4", 64)); err == nil {
			t.Fatal("recovery accepted a partial original transcript as complete")
		}
	}
}

func TestSourceRecoveryRecordsRejectEnvelopeWithWrongSemanticTail(t *testing.T) {
	full, binding, hashes := sourceReplacementRecordVector(t)
	selected := sourceReplacementBindingVector()
	records, err := decodeSourceRecoveryPrefix(full, 55)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.SplitAfter(full, []byte{'\n'})
	prefix := bytes.Join(lines[:54], nil)
	tail := records[54]
	tail.Phase = "stopped-verified" // Valid initial phase, invalid replacement completion.
	line, err := json.Marshal(tail)
	if err != nil {
		t.Fatal(err)
	}
	wrong := append(bytes.Clone(prefix), append(line, '\n')...)
	completion := sourceRecoveryCompletion{Schema: "ardents-source-recovery-completion-v1", Operation: "replacement",
		Journal: records[0].Path, Device: records[0].Device, Inode: records[0].Inode,
		PrefixSHA256: digest(prefix), PrefixRecords: 54, Generation: binding.Configuration.Generation,
		BindingSHA256: digest(selected), Records: sourceRecoveryRecords{tail}, TranscriptSHA256: digest(wrong)}
	info := sourceRecoveryCodecInfo{size: int64(len(prefix)), mode: 0600,
		stat: syscall.Stat_t{Dev: records[0].Device, Ino: records[0].Inode, Nlink: 1}}
	transcript, err := decodeSourceRecoveryCompletion(marshalSourceRecoveryCodec(t, completion), prefix, info,
		completion.Journal, completion.Operation, completion.Generation, completion.BindingSHA256)
	if err != nil {
		t.Fatal("fixture did not isolate semantic validation after a consistent envelope", err)
	}
	if _, err := decodeSourceRecoveredRecords(transcript, selected, "replacement", hashes, strings.Repeat("3", 64), strings.Repeat("4", 64)); err == nil {
		t.Fatal("completion envelope bypassed the closed replacement grammar")
	}
	for _, operation := range []string{"initial", "start", "retirement", "unknown"} {
		if _, err := decodeSourceRecoveredRecords(full, selected, operation, hashes, strings.Repeat("3", 64), strings.Repeat("4", 64)); err == nil {
			t.Fatal("recovery reinterpreted the original operation", operation)
		}
	}
}
