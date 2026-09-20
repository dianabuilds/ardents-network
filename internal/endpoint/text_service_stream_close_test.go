//go:build linux

package endpoint

import (
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTextServiceCloseDrainsOppositeDirectionBeforeFullClose(t *testing.T) {
	application, native := newApplicationHalfClosePair()
	finished := make(chan struct{})
	stream := &textServiceStream{applicationHalfClose: application, cancel: func() {}, retired: finished, finished: finished,
		waitClose: func(done <-chan struct{}) bool { <-done; return true }}
	read := make(chan struct {
		body []byte
		err  error
	}, 1)
	go func() {
		body, err := io.ReadAll(stream)
		read <- struct {
			body []byte
			err  error
		}{body: body, err: err}
	}()
	nativeDone := make(chan error, 1)
	go func() {
		var one [1]byte
		_, err := native.Read(one[:])
		if err != io.EOF {
			nativeDone <- err
			return
		}
		if _, err = native.Write([]byte("tail")); err == nil {
			err = native.CloseInput()
		}
		result := <-read
		if err == nil {
			err = result.err
		}
		if err == nil && string(result.body) != "tail" {
			err = io.ErrUnexpectedEOF
		}
		close(finished)
		nativeDone <- err
	}()
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-nativeDone; err != nil {
		t.Fatal(err)
	}
}

func TestTextServiceCloseCancelsAndJoinsAfterGraceExpires(t *testing.T) {
	application, native := newApplicationHalfClosePair()
	defer native.Close()
	retired := make(chan struct{})
	finished := make(chan struct{})
	var canceled atomic.Bool
	stream := &textServiceStream{applicationHalfClose: application, retired: retired, finished: finished,
		waitClose: func(<-chan struct{}) bool { return false }, cancel: func() {
			if canceled.CompareAndSwap(false, true) {
				close(finished)
			}
		}}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if !canceled.Load() {
		t.Fatal("expired close grace did not cancel the native stream")
	}
	var one [1]byte
	if _, err := native.Read(one[:]); err != io.EOF {
		t.Fatalf("fallback canceled before directional EOF: %v", err)
	}
}

func TestTextServiceCloseReleasesTailAfterBoundedRetirement(t *testing.T) {
	application, native := newApplicationHalfClosePair()
	defer native.Close()
	retired := make(chan struct{})
	finished := make(chan struct{})
	var premature atomic.Bool
	stream := &textServiceStream{applicationHalfClose: application, retired: retired, finished: finished,
		waitClose: func(done <-chan struct{}) bool { <-done; return true }, cancel: func() {
			select {
			case <-retired:
			default:
				premature.Store(true)
			}
		}, retireTail: func() error {
			select {
			case <-retired:
			default:
				premature.Store(true)
			}
			close(finished)
			return nil
		}}
	close(retired)
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if premature.Load() {
		t.Fatal("terminal-control tail was canceled before bounded retirement")
	}
}

func TestTextServiceCloseInterruptsAbandonedUnreadResponse(t *testing.T) {
	application, native := newApplicationHalfClosePair()
	retired := make(chan struct{})
	finished := make(chan struct{})
	var canceled atomic.Bool
	var cancelOnce sync.Once
	cancel := func() {
		cancelOnce.Do(func() {
			canceled.Store(true)
			_ = native.Close()
			close(finished)
		})
	}
	stream := &textServiceStream{applicationHalfClose: application, retired: retired, finished: finished, cancel: cancel}
	written := make(chan error, 1)
	go func() { _, err := native.Write([]byte("unread response")); written <- err }()
	closed := make(chan error, 1)
	go func() { closed <- stream.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		cancel()
		<-closed
		t.Fatal("full Close did not interrupt an abandoned unread response")
	}
	if !canceled.Load() {
		t.Fatal("full Close completed without canceling unfinished Application I/O")
	}
	if err := <-written; err == nil {
		t.Fatal("abandoned native response write reported success")
	}
}
