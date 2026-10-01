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
