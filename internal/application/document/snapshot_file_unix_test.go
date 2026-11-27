//go:build linux || darwin

package document

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

// unixTempDir resolves the platform temp root to its real path: on macOS the
// system temp directory lives behind the /var symlink, and the no-follow import
// must refuse symlinked ancestors rather than traverse them.
func unixTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSnapshotFileImportsStableBoundedUTF8WithoutSharingFile(t *testing.T) {
	for _, body := range [][]byte{nil, []byte("text\nПривет\u202e"), bytes.Repeat([]byte("x"), MaximumBytes)} {
		path := filepath.Join(unixTempDir(t), "document")
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		imported, err := ReadSnapshotFile(t.Context(), path)
		if err != nil || !bytes.Equal(imported, body) {
			t.Fatalf("import %d: %v", len(body), err)
		}
		if err := os.WriteFile(path, []byte("changed after import"), 0600); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(imported, body) {
			t.Fatal("file mutation changed retained snapshot")
		}
	}
}
func TestSnapshotFileRefusesSymlinksSpecialFilesAndInvalidContent(t *testing.T) {
	root := unixTempDir(t)
	regular := filepath.Join(root, "regular")
	if err := os.WriteFile(regular, []byte("valid"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	directoryLink := filepath.Join(root, "directory-link")
	if err := os.Symlink(root, directoryLink); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(root, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(root, "invalid")
	if err := os.WriteFile(invalid, []byte{0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	if body, err := ReadSnapshotFile(t.Context(), invalid); err != nil || !bytes.Equal(body, []byte{0xff}) {
		t.Fatalf("binary snapshot = %x, %v", body, err)
	}
	large := filepath.Join(root, "large")
	if err := os.WriteFile(large, bytes.Repeat([]byte("x"), MaximumBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, filepath.Join(directoryLink, "regular"), root, fifo, "/dev/null", large, "relative", regular + "/../regular"} {
		body, err := ReadSnapshotFile(t.Context(), path)
		if err == nil || body != nil {
			t.Fatalf("invalid input admitted: %s", filepath.Base(path))
		}
		if err != nil && bytes.Contains([]byte(err.Error()), []byte(root)) {
			t.Fatal("diagnostic exposed selected path")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if body, err := ReadSnapshotFile(ctx, regular); body != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled import: %v", err)
	}
}
func TestSnapshotFileRejectsChangingInputEvenWithRestoredSizeAndMtime(t *testing.T) {
	path := filepath.Join(unixTempDir(t), "document")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := openSnapshotFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	// An immediate rewrite can share the kernel's coarse ctime tick (Linux
	// ext4); repeat the mutation until the change time observably advances so
	// the refusal rests on real mutation evidence (generation native idiom).
	born, ok := durablefile.InodeChangeTime(before)
	if !ok {
		t.Fatal("original change time unavailable")
	}
	deadline := time.Now().Add(time.Second)
	var after os.FileInfo
	for {
		if err := os.WriteFile(path, []byte("after!"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
			t.Fatal(err)
		}
		after, err = file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if current, ok := durablefile.InodeChangeTime(after); ok && current != born {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatal("invalid environment: no observable native ctime transition")
		}
		time.Sleep(time.Millisecond)
	}
	if before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		t.Fatal("mutation positive control failed")
	}
	if err := verifySnapshotFile(t.Context(), file, before, body); err == nil {
		t.Fatal("same-size rewritten input with restored mtime accepted")
	}
}
