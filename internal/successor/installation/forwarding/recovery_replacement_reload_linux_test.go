package forwarding

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Synthetic file metadata exercises canonical bytes only; no original native
// scope, manager, Release, Reload or recovery admission follows from this type.
type reloadFileVector struct {
	size          int64
	device, inode uint64
}

func (v reloadFileVector) Name() string       { return "codec-native-after" }
func (v reloadFileVector) Size() int64        { return v.size }
func (v reloadFileVector) Mode() os.FileMode  { return 0600 }
func (v reloadFileVector) ModTime() time.Time { return time.Time{} }
func (v reloadFileVector) IsDir() bool        { return false }
func (v reloadFileVector) Sys() any           { return &syscall.Stat_t{Dev: v.device, Ino: v.inode, Nlink: 1} }

func replacementPreReloadVector(t *testing.T) ([]initialRecord, *recoveryCandidate, generationSelection, map[string]string, transitionCutResult, []byte, []byte) {
	t.Helper()
	rows, selected, previous, hashes, before, after := replacementContextVector(t)
	native, err := decodeNativeContext(before)
	if err != nil {
		t.Fatal(err)
	}
	cut := transitionCutResult{Boundary: transitionMessage{Operation: "replacement", Journal: rows[0].Path, Device: rows[0].Device, Inode: rows[0].Inode, Generation: selected.binding.Generation, BindingSHA256: digest(selected.raw), Binding: selected.raw, OriginalContext: before, NativeContext: before, Manager: native.Manager}, Origin: transitionOrigin{ProgramSHA256: selected.binding.ProgramSHA256}}
	return rows, &recoveryCandidate{selection: selected}, previous, hashes, cut, before, after
}

func TestForwardingReplacementPreReloadInventoryIsNotCompletedHistory(t *testing.T) {
	rows, candidate, previous, hashes, cut, before, after := replacementPreReloadVector(t)
	for count := 1; count <= 62; count++ {
		prefix := encodeReplacementVector(t, rows[:count])
		cut.Boundary.Records = uint32(count)
		cut.Boundary.PrefixSHA256 = digest(prefix)
		owned, err := replacementPreReloadInventory(prefix, cut, candidate.selection, previous, hashes, before)
		if count != 57 && count != 58 {
			if err == nil {
				t.Fatal("unproved original boundary", count)
			}
			continue
		}
		if err != nil || len(owned) != 20 {
			t.Fatal("complete original prefix inventory", count, len(owned), err)
		}
		if _, err := decodeReplacementContextRecords(prefix, candidate.selection, previous, hashes, before, after); err == nil {
			t.Fatal("prefix supplied complete replacement", count)
		}
		if _, err := stoppedReplacementTranscript(prefix, cut, candidate.selection, previous, hashes, before, after); err == nil {
			t.Fatal("prefix supplied original stopped result", count)
		}
	}
	for index := range rows[:57] {
		bad := append([]initialRecord(nil), rows[:57]...)
		bad[index].Path += "-foreign"
		prefix := encodeReplacementVector(t, bad)
		cut.Boundary.Records = 57
		cut.Boundary.PrefixSHA256 = digest(prefix)
		if _, err := replacementPreReloadInventory(prefix, cut, candidate.selection, previous, hashes, before); err == nil {
			t.Fatal("foreign original path", index)
		}
	}
	prefix := encodeReplacementVector(t, rows[:58])
	cut.Boundary.Records = 58
	cut.Boundary.PrefixSHA256 = digest(prefix)
	cut.Boundary.NativeContext = after
	if _, err := replacementPreReloadInventory(prefix, cut, candidate.selection, previous, hashes, before); err == nil {
		t.Fatal("already changed context supplied first Reload right")
	}
}

func TestForwardingReplacementReloadCompletionOwnsCanonicalAfterBirth(t *testing.T) {
	rows, candidate, previous, hashes, cut, before, after := replacementPreReloadVector(t)
	beforePath, afterPath := replacementNativePaths(cut.Boundary.Journal)
	beforeFile := &initialFile{path: beforePath, body: before}
	afterFile := &initialFile{path: afterPath, body: after, info: reloadFileVector{int64(len(after)), rows[59].Device, rows[59].Inode}}
	for _, count := range []int{57, 58} {
		prefix := encodeReplacementVector(t, rows[:count])
		cut.Boundary.Records = uint32(count)
		cut.Boundary.PrefixSHA256 = digest(prefix)
		original := &recoveryOriginal{cut: cut, prefix: &transitionPrefix{journal: &initialFile{path: cut.Boundary.Journal, body: bytes.Clone(prefix)}}, result: &initialFile{body: []byte("detached cut byte vector\n")}}
		raw, err := replacementReloadCompletion(original, candidate, previous, hashes, beforeFile, afterFile)
		if err != nil {
			t.Fatal(count, err)
		}
		completion, full, err := decodeRecoveryCompletion(raw, prefix, original.result.body, cut)
		if err != nil || completion.NativeContextSHA256 != digest(after) || completion.Schema != "ardents-forwarding-recovery-completion-v2" || len(completion.Records) != 62-count || !bytes.Equal(full, encodeReplacementVector(t, rows)) || !bytes.Equal(original.prefix.journal.body, prefix) {
			t.Fatal("separate continuation or original changed", count, err)
		}
		position := 0
		if count == 57 {
			position = 1
		}
		var born initialRecord
		if json.Unmarshal(completion.Records[position+1], &born) != nil || born.Phase != "file-born" || born.Path != afterPath || born.Inode != rows[59].Inode {
			t.Fatal("recovery after birth not separately committed")
		}
		wrong := *afterFile
		wrong.path += "-foreign"
		if _, err := replacementReloadCompletion(original, candidate, previous, hashes, beforeFile, &wrong); err == nil {
			t.Fatal("noncanonical after file admitted")
		}
	}
}

func TestForwardingReplacementPreReloadRefusesUnrecordedAfter(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "original-journal")
	before := []byte("physical absence check only\n")
	original := &recoveryOriginal{cut: transitionCutResult{Boundary: transitionMessage{NativeContext: before}}}
	r := &recoveredReplacement{journal: &initialFile{path: journal}, beforeReload: &initialFile{body: before}, owner: &completedFiles{}, original: original}
	if err := r.checkBeforeReloadAfter(); err != nil {
		t.Fatal("actual original after absence", err)
	}
	_, after := replacementNativePaths(journal)
	if err := os.WriteFile(after, []byte("unknown original birth\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.checkBeforeReloadAfter(); err == nil {
		t.Fatal("existing after leaf adopted")
	}
	body, err := os.ReadFile(after)
	if err != nil || string(body) != "unknown original birth\n" {
		t.Fatal("unrecorded birth modified", err)
	}
}
