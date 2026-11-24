//go:build linux || darwin

package durablefile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mutateUntilChangeTimeAdvances repeats one mutation until the filesystem's
// change time observably advances and returns the fresh observation. An
// immediate mutation pair can share the kernel's coarse ctime tick (Linux
// ext4) and would supply no mutation evidence; the bounded wait matches the
// generation native ctime-transition idiom.
func mutateUntilChangeTimeAdvances(t *testing.T, path string, before os.FileInfo, mutate func() error) os.FileInfo {
	t.Helper()
	born, ok := InodeChangeTime(before)
	if !ok {
		t.Fatal("original change time unavailable")
	}
	deadline := time.Now().Add(time.Second)
	for {
		if err := mutate(); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if current, ok := InodeChangeTime(info); ok && current != born {
			return info
		}
		if !time.Now().Before(deadline) {
			t.Fatal("invalid environment: no observable native ctime transition")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSameFileIdentityRetainsOneUnchangedInode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	named, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !SameFileIdentity(named, opened) || !SameFileIdentity(opened, named) {
		t.Fatal("one unchanged inode refused")
	}
}

// The change time must advance on a same-size content rewrite even when the
// modification time is restored: this is the anti-substitution signal on both
// Linux (ext4) and macOS (APFS) filesystems.
func TestSameFileIdentityRefusesRewriteWithRestoredModificationTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	after := mutateUntilChangeTimeAdvances(t, path, before, func() error {
		if err := os.WriteFile(path, []byte("after!"), 0600); err != nil {
			return err
		}
		return os.Chtimes(path, before.ModTime(), before.ModTime())
	})
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("mutation positive control failed")
	}
	if SameFileIdentity(before, after) {
		t.Fatal("same-size rewritten input with restored mtime accepted")
	}
}

func TestSameFileIdentityRefusesLinkAccessAndSubstitution(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "record")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	linked, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if SameFileIdentity(before, linked) {
		t.Fatal("multiply-linked inode accepted")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	unlinked := mutateUntilChangeTimeAdvances(t, path, before, func() error {
		if err := os.Link(path, link); err != nil {
			return err
		}
		return os.Remove(link)
	})
	if SameFileIdentity(before, unlinked) {
		t.Fatal("link-count mutation with restored count accepted")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	widened, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if SameFileIdentity(before, widened) {
		t.Fatal("widened access accepted")
	}
	substitute := filepath.Join(root, "substitute")
	if err := os.WriteFile(substitute, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	other, err := os.Lstat(substitute)
	if err != nil {
		t.Fatal(err)
	}
	if SameFileIdentity(before, other) {
		t.Fatal("distinct inode with equal bytes and access accepted")
	}
	if SameFileIdentity(nil, before) || SameFileIdentity(before, nil) || SameFileIdentity(nil, nil) {
		t.Fatal("absent observation accepted")
	}
}

func TestInodeTimesNormalizeTheStatObservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	mtime, ok := InodeModificationTime(info)
	if !ok || mtime.Sec != info.ModTime().Unix() || mtime.Nsec != int64(info.ModTime().Nanosecond()) {
		t.Fatalf("modification time normalization differs: %v vs %v", mtime, info.ModTime())
	}
	ctime, ok := InodeChangeTime(info)
	if !ok || (ctime.Sec == 0 && ctime.Nsec == 0) {
		t.Fatalf("change time unavailable: %v", ctime)
	}
	if _, ok := InodeChangeTime(nil); ok {
		t.Fatal("nil observation admitted")
	}
	if _, ok := InodeModificationTime(nil); ok {
		t.Fatal("nil observation admitted")
	}
	changed := mutateUntilChangeTimeAdvances(t, path, info, func() error {
		return os.Chmod(path, 0640)
	})
	after, ok := InodeChangeTime(changed)
	if !ok || after == ctime {
		t.Fatal("metadata mutation did not advance the change time")
	}
}
