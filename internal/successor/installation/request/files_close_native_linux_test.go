//go:build linux && network_signing_native

package request

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
)

func TestDeliveryFilesNativeCleanCloseDeniesHeldReadWithoutChangingJoinedResult(t *testing.T) {
	files, _ := prepareDeliveryFiles(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	files.original.ctx = ctx
	entered, release, denied := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var readOnce, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	files.original.observe = func(stage string, _ *os.File) {
		if stage == "read" {
			readOnce.Do(func() { close(entered); <-release })
		}
		if stage == "denied" {
			close(denied)
		}
	}
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- files.Check() }()
	<-entered
	go func() { closed <- files.Close() }()
	<-denied
	select {
	case <-closed:
		t.Fatal("clean Close bypassed the original held reader")
	default:
	}
	unblock()
	if err := <-checked; !errors.Is(err, errFilesClosed) {
		t.Fatal("held reader handed off after Close admission sealed", err)
	}
	if err := <-closed; err != nil {
		t.Fatal("own closed admission poisoned clean physical completion", err)
	}
	cancel()
	if files.Check() == nil {
		t.Fatal("completed/canceled custody accepted a late observation")
	}
	if err := files.Close(); err != nil {
		t.Fatal("late caller cancellation rewrote the clean joined result", err)
	}
}
