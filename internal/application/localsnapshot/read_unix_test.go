//go:build linux || darwin

package localsnapshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/document"
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

func TestReadReturnsDetachedBytesIncludingBinaryContent(t *testing.T) {
	for _, body := range [][]byte{nil, {0xff}, []byte("text\nПривет\u202e"), bytes.Repeat([]byte("x"), document.MaximumBytes)} {
		path := filepath.Join(unixTempDir(t), "document")
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		imported, err := Read(t.Context(), path)
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

func TestReadRefusesUnsafePathsSpecialFilesAndOversizedInput(t *testing.T) {
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
	large := filepath.Join(root, "large")
	if err := os.WriteFile(large, bytes.Repeat([]byte("x"), document.MaximumBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, filepath.Join(directoryLink, "regular"), root, fifo, "/dev/null", large, "relative", regular + "/../regular"} {
		body, err := Read(t.Context(), path)
		if err == nil || body != nil {
			t.Fatalf("invalid input admitted: %s", filepath.Base(path))
		}
		if err != nil && bytes.Contains([]byte(err.Error()), []byte(root)) {
			t.Fatal("diagnostic exposed selected path")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if body, err := Read(ctx, regular); body != nil || !errors.Is(err, context.Canceled) {
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
	digest := sha256.Sum256(body)
	if err := verifySnapshotFile(t.Context(), file, before, digest[:]); err == nil {
		t.Fatal("same-size rewritten input with restored mtime accepted")
	}
}

func TestVerificationRejectsDifferentBytesWithMatchingMetadata(t *testing.T) {
	path := filepath.Join(unixTempDir(t), "document")
	if err := os.WriteFile(path, []byte("later"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := openSnapshotFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	original, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	// Both metadata observations see the same file. Use an independently
	// computed digest for different bytes of the same length.
	firstDigest := sha256.Sum256([]byte("first"))
	if err := verifySnapshotFile(t.Context(), file, original, firstDigest[:]); !errors.Is(err, errSnapshotFile) {
		t.Fatalf("second pass with different bytes = %v, want refusal", err)
	}
}

func TestSnapshotPassCopiesBeyondCurrentPublicationLimit(t *testing.T) {
	body := bytes.Repeat([]byte{0x00, 0x80, 0xff}, document.MaximumBytes/3+1)
	want := sha256.Sum256(body)
	output := sha256.New()
	if err := copySnapshotPass(t.Context(), bytes.NewReader(body), output, int64(len(body))); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Sum(nil), want[:]) {
		t.Fatal("snapshot pass altered the binary input")
	}
}

func TestSnapshotPassRejectsShortAndSurplusInput(t *testing.T) {
	for _, input := range []string{"short", "surplus"} {
		if err := copySnapshotPass(t.Context(), strings.NewReader(input), io.Discard, 6); !errors.Is(err, errSnapshotFile) {
			t.Fatalf("snapshot pass for %q = %v, want refusal", input, err)
		}
	}
}
