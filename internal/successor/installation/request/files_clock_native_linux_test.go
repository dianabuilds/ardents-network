//go:build linux && network_signing_native

package request

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDeliveryFilesNativeClockCloseJoinsWithoutPoisoningCleanResult(t *testing.T) {
	for _, fault := range []bool{false, true} {
		name := "clean"
		if fault {
			name = "retained-physical-close-failure"
		}
		t.Run(name, func(t *testing.T) { observeOriginalClockClose(t, fault) })
	}
}

func observeOriginalClockClose(t *testing.T, closeFault bool) {
	t.Helper()
	files, path := prepareDeliveryFiles(t)
	clock := filepath.Join(filepath.Dir(path), "clock")
	if err := os.WriteFile(clock, []byte("live root observation\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := files.RetainParents(clock); err != nil {
		t.Fatal(err)
	}
	entered, release, denied := make(chan *os.File, 1), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	files.original.observe = func(stage string, file *os.File) {
		if stage == "clock" {
			entered <- file
			<-release
		}
		if stage == "denied" {
			close(denied)
		}
	}
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- files.CheckClock(clock) }()
	original := <-entered
	go func() { closed <- files.Close() }()
	<-denied
	if _, err := original.Stat(); err != nil {
		t.Fatal("clock custody returned before original observation joined", err)
	}
	select {
	case <-closed:
		t.Fatal("Close bypassed original clock observer")
	default:
	}
	if closeFault {
		if err := original.Close(); err != nil {
			t.Fatal(err)
		}
	}
	unblock()
	if err := <-checked; !errors.Is(err, errFilesClosed) {
		t.Fatal("closing custody accepted a clock observation", err)
	}
	joined := <-closed
	if !closeFault && joined != nil || errors.Is(joined, os.ErrClosed) != closeFault {
		t.Fatal("own clock denial or independent physical Close result differs", joined)
	}
	if _, err := original.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("original clock descriptor survived joined Close", err)
	}
	if err := files.Close(); err != joined {
		t.Fatal("repeated clock Close changed retained result", err)
	}
}
