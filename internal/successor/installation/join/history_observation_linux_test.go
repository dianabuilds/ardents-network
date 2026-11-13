package join

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// A failed/held history observer supplies no accepting authority. Even this
// rejected use must retain the actual original journal descriptor through join.
func TestHistoryObservationRetainsOriginalReaderUntilClose(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "original-journal")
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	c := &completedInitial{ctx: ctx, parents: &nativeDirectories{ctx: ctx}, journal: &initialFile{path: path, file: file}}
	h := Completed{c}
	copyHandle := h
	entered, release, checked, closed := make(chan struct{}), make(chan struct{}), make(chan error, 1), make(chan error, 1)
	refusal := errors.New("original history observer deliberately refused")
	var calls atomic.Int32
	go func() {
		checked <- h.operationObserved(func() error { calls.Add(1); close(entered); <-release; return refusal }, nil)
	}()
	<-entered
	go func() { closed <- copyHandle.Close() }()
	until := time.Now().Add(time.Second)
	for {
		c.mu.Lock()
		denied := c.closed
		c.mu.Unlock()
		if denied {
			break
		}
		if time.Now().After(until) {
			close(release)
			t.Fatal("Close did not seal original reader")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case err := <-closed:
		close(release)
		t.Fatal("Close returned while history observer was retained", err)
	default:
	}
	if _, err := file.Stat(); err != nil {
		close(release)
		t.Fatal("original journal released early", err)
	}
	if err := h.operationObserved(func() error { calls.Add(1); return context.Canceled }, nil); err != errCompletedClosed {
		close(release)
		t.Fatal("late observation admitted", err)
	}
	if calls.Load() != 1 {
		close(release)
		t.Fatal("Close admitted a later observer")
	}
	close(release)
	if err := <-checked; err != refusal {
		t.Fatal("original refusal changed", err)
	}
	if err := <-closed; !errors.Is(err, refusal) {
		t.Fatal("Close lost original observation failure", err)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("journal not released after actual observer join", err)
	}
	if err := h.Close(); !errors.Is(err, refusal) {
		t.Fatal("copied Close lost retained failure", err)
	}
}
