//go:build linux

package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSuccessorReplacementRepairsOnlyRecordedOriginalInode(t *testing.T) {
	if os.Geteuid() != 0 {
		if err := replaceTransitionFile(context.Background(), "", []byte("old"), []byte("new"), 0644, 0, "", selection{}); err == nil {
			t.Fatal("non-root replacement accepted")
		}
		return // Root-filesystem success is covered only by the selected root run.
	}
	base := replacementTestRoot(t)
	path, record := filepath.Join(base, "fixed-resource"), filepath.Join(base, "copy.json")
	old, next := []byte("complete old resource"), []byte("complete successor resource")
	selected := selection{GenerationDigest: digestHex(next), BindingDigest: digestHex([]byte("binding"))}
	if err := writeExclusiveGenerationFile(path, old, 0644, 0); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceTransitionFile(context.Background(), path, old, next, 0644, 0, record, selected); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) {
		t.Fatal("replacement changed the direct installed inode")
	}
	// A durable preimage record owns this exact torn write, not any pathname
	// that appears later with the same target name.
	if err := os.WriteFile(path, next[:5], 0644); err != nil {
		t.Fatal(err)
	}
	if err := replaceTransitionFile(context.Background(), path, old, next, 0644, 0, record, selected); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !bytes.Equal(body, next) {
		t.Fatal("recorded torn write was not completed")
	}
	// Death after truncate, before the first successor byte, leaves an empty
	// prefix on the recorded inode. Both recovery's reader and writer must admit
	// that prefix without extending the same allowance to unrecorded files.
	if err := os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	emptyInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := readRecordedReplacementBytes(path, emptyInfo)
	if err != nil || len(prefix) != 0 {
		t.Fatal("recorded zero-byte prefix unavailable for recovery validation", err)
	}
	if err := replaceTransitionFile(context.Background(), path, old, next, 0644, 0, record, selected); err != nil {
		t.Fatal("recorded truncate-before-write interruption refused", err)
	}
	body, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(body, next) {
		t.Fatal("recorded zero-byte prefix was not completed", err)
	}
	if err := os.WriteFile(path, []byte("foreign bytes on recorded inode"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := replaceTransitionFile(context.Background(), path, old, next, 0644, 0, record, selected); err == nil {
		t.Fatal("replacement journal accepted foreign bytes on its recorded inode")
	}
	body, _ = os.ReadFile(path)
	if !bytes.Equal(body, []byte("foreign bytes on recorded inode")) {
		t.Fatal("refused replacement overwrote foreign bytes")
	}
	if err := os.Rename(path, filepath.Join(base, "original-inode")); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusiveGenerationFile(path, []byte("foreign replacement"), 0644, 0); err != nil {
		t.Fatal(err)
	}
	if err := replaceTransitionFile(context.Background(), path, old, next, 0644, 0, record, selected); err == nil {
		t.Fatal("replacement journal adopted another inode")
	}
	body, _ = os.ReadFile(path)
	if !bytes.Equal(body, []byte("foreign replacement")) {
		t.Fatal("foreign inode changed before refusal")
	}
}

func TestSuccessorReplacementRefusesForeignPreimageAndCancellation(t *testing.T) {
	if os.Geteuid() != 0 {
		return // Other test explicitly verifies the non-root refusal boundary.
	}
	base := replacementTestRoot(t)
	path, record := filepath.Join(base, "fixed-resource"), filepath.Join(base, "copy.json")
	if err := writeExclusiveGenerationFile(path, []byte("foreign"), 0644, 0); err != nil {
		t.Fatal(err)
	}
	if err := replaceTransitionFile(context.Background(), path, []byte("old"), []byte("new"), 0644, 0, record, selection{}); err == nil {
		t.Fatal("foreign preimage accepted")
	}
	if _, err := os.Lstat(record); !os.IsNotExist(err) {
		t.Fatal("foreign preimage created a replacement journal")
	}
	if err := os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	if err := replaceTransitionFile(context.Background(), path, []byte("old"), []byte("new"), 0644, 0, record, selection{}); err == nil {
		t.Fatal("empty unrecorded file adopted as an owned interrupted write")
	}
	if _, err := os.Lstat(record); !os.IsNotExist(err) {
		t.Fatal("unrecorded empty preimage created a replacement journal")
	}
	if body, err := os.ReadFile(path); err != nil || len(body) != 0 {
		t.Fatal("refusal changed unrecorded empty file", err)
	}
	if err := os.WriteFile(path, []byte("foreign"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := replaceTransitionFile(ctx, path, []byte("foreign"), []byte("new"), 0644, 0, record, selection{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled replacement changed a file")
	}
	body, _ := os.ReadFile(path)
	if !bytes.Equal(body, []byte("foreign")) {
		t.Fatal("pre-effect refusal changed the original bytes")
	}
}

func TestSuccessorRetainsFirstFailureAcrossExplicitRecoveryAttempt(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	base := replacementTestRoot(t)
	selected := selection{GenerationDigest: digestHex([]byte("generation")), BindingDigest: digestHex([]byte("binding"))}
	if err := retainTransitionFailure(base, selected, errors.New("original stop refused")); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(base, "transition-failure.json"))
	if err := retainTransitionFailure(base, selected, errors.New("later attempt refused")); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(base, "transition-failure.json"))
	if !bytes.Equal(before, after) || !bytes.Contains(after, []byte("original stop refused")) {
		t.Fatal("recovery erased the first failure")
	}
}

func replacementTestRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/root", "ardents-successor-component-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	return root
}

func TestRecordedReplacementRequiresJournalDurabilityBeforeMutation(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	journal := filepath.Join(root, "journal")
	if err := os.Mkdir(journal, 0700); err != nil {
		t.Fatal(err)
	}
	path, record := filepath.Join(root, "resource"), filepath.Join(journal, "copy.json")
	old, next := []byte("complete previous"), []byte("complete successor")
	selected := selection{GenerationDigest: digestHex(next)}
	if err := writeExclusiveGenerationFile(path, old, 0644, 0); err != nil {
		t.Fatal(err)
	}
	refusal := errors.New("original replacement journal sync refused")
	syncJournal := func(directory string) error {
		if directory == journal {
			return refusal
		}
		return syncDirectory(directory)
	}
	if err := replaceTransitionFileWithSync(context.Background(), path, old, next, 0644, 0, record, selected, syncJournal); !errors.Is(err, refusal) {
		t.Fatal("initial journal durability refusal lost", err)
	}
	if body, err := os.ReadFile(path); err != nil || !bytes.Equal(body, old) {
		t.Fatal("initial sync refusal changed resource", err)
	}
	original, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	err = replaceTransitionFileWithSync(context.Background(), path, old, next, 0644, 0, record, selected, syncJournal)
	if !errors.Is(err, refusal) {
		t.Fatal("visible journal bypassed required durability refusal", err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(body, old) {
		t.Fatal("resource changed before journal durability", err)
	}
	retained, err := os.ReadFile(record)
	if err != nil || !bytes.Equal(retained, original) {
		t.Fatal("durability refusal changed owned journal", err)
	}
}
