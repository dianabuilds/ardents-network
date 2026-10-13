//go:build linux

package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Local pipes isolate the fixed Application projection only. These callbacks
// provide no qualified Job, Source, authentication or installed readiness.
func TestInstalledReaderProjectionRejectsMalformedRequestBeforeWorkerTransfer(t *testing.T) {
	var transfers, retirements atomic.Int32
	stream := newInstalledReaderResult(t.Context(), func() error { return nil }, func() ([]byte, error) {
		transfers.Add(1)
		return nil, errors.New("unexpected worker transfer")
	}, func() error { retirements.Add(1); return nil })
	defer stream.Close()
	if _, err := stream.Write([]byte("invalid")); err != nil {
		t.Fatal(err)
	}
	_ = stream.CloseInput()
	var body [1]byte
	if n, err := stream.Read(body[:]); n != 0 || err == nil {
		t.Fatal("malformed local request exposed a response", n, err)
	}
	if err := stream.Close(); err == nil || transfers.Load() != 0 || retirements.Load() != 1 {
		t.Fatal("malformed request reached worker or lost retirement", err, transfers.Load(), retirements.Load())
	}
}

func TestInstalledReaderProjectionCloseInterruptsPendingOriginalCompletion(t *testing.T) {
	entered, released := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	var retirements atomic.Int32
	stream := newInstalledReaderResult(t.Context(), func() error { return nil }, func() ([]byte, error) {
		close(entered)
		<-released
		return []byte("late private result"), nil
	}, func() error {
		retirements.Add(1)
		releaseOnce.Do(func() { close(released) })
		return nil
	})
	defer stream.Close()
	request := make([]byte, 512)
	copy(request, "ARDTXT01")
	request[8] = 1
	if _, err := stream.Write(request); err != nil {
		t.Fatal(err)
	}
	_ = stream.CloseInput()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("fixed local request did not reach completion")
	}
	closed := make(chan error, 1)
	go func() { closed <- stream.Close() }()
	select {
	case err := <-closed:
		if !errors.Is(err, context.Canceled) || retirements.Load() == 0 {
			t.Fatal("original pending completion survived local Close", err, retirements.Load())
		}
	case <-time.After(time.Second):
		releaseOnce.Do(func() { close(released) })
		t.Fatal("Close did not interrupt and join original completion")
	}
	var body [1]byte
	if n, _ := stream.Read(body[:]); n != 0 {
		t.Fatal("late result escaped after local Close")
	}
}
