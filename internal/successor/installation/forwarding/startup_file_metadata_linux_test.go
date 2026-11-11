package forwarding

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Actual Linux file metadata, not accepting Startup or manager authority.
func TestStartupOriginalFileCannotReviveAfterBytesAndMtimeRestoration(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "original")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	body := []byte("original physical bytes")
	if _, err := file.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	before, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	// Cross a real kernel timestamp tick before the causal mutation. A Go
	// virtual clock cannot advance the filesystem's actual ctime.
	time.Sleep(2 * time.Millisecond)
	for _, value := range []byte{body[0] ^ 1, body[0]} {
		if _, err := file.WriteAt([]byte{value}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file.Name(), before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	a, b := before.Sys().(*syscall.Stat_t), after.Sys().(*syscall.Stat_t)
	restored, err := os.ReadFile(file.Name())
	if err != nil || !bytes.Equal(restored, body) || !os.SameFile(before, after) || a.Mtim != b.Mtim || a.Ctim == b.Ctim {
		t.Fatal("actual byte/mtime restoration with changed original ctime not established", err)
	}
	if sameInputFile(before, after) {
		t.Fatal("restored bytes and mtime revived immutable original metadata")
	}
	parent, err := os.Open(filepath.Dir(file.Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	named, err := statCompletedName(&inputDirectory{file: parent}, filepath.Base(file.Name()), false)
	if err != nil || sameCompletedFile(before, named) {
		t.Fatal("fresh named metadata revived restored bytes and mtime", err)
	}
}
