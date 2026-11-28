//go:build linux || darwin

package publisher

import (
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"testing/synctest"
)

func TestSnapshotSourceIsRetainedUntilPublicationServingJoins(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	const body = "selected bytes"
	if _, err := file.WriteString(body); err != nil {
		t.Fatal(err)
	}

	synctest.Test(t, func(t *testing.T) {
		released := false
		source := &snapshotSource{file: file, size: int64(len(body)), release: func() { released = true }}
		served := make(chan struct{})
		closed := make(chan error, 1)
		publication := &published{snapshot: source, cancel: func() {}, started: true, served: served}
		go func() { closed <- publication.Close() }()
		synctest.Wait()

		if released {
			t.Error("storage reservation released before the serving borrower joined")
		}
		first, second := source.openReader(), source.openReader()
		prefix := make([]byte, 2)
		if _, err := io.ReadFull(first, prefix); err != nil {
			t.Error(err)
		}
		if got, err := io.ReadAll(second); err != nil || string(got) != body {
			t.Errorf("independent reader = %q, %v", got, err)
		}
		close(served)
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
		if !released {
			t.Fatal("joined Publication retained the storage reservation")
		}
		if _, err := source.openReader().Read(prefix); !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EBADF) {
			t.Fatalf("retired source read error = %v", err)
		}
	})
}
