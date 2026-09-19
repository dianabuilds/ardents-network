//go:build linux

package endpoint

import (
	"io"
	"sync/atomic"
	"testing"
)

func TestTextServiceCloseDrainsOppositeDirectionBeforeFullClose(t *testing.T) {
	application, native := newApplicationHalfClosePair()
	finished := make(chan struct{})
	stream := &textServiceStream{applicationHalfClose: application, cancel: func() {}, finished: finished,
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
	finished := make(chan struct{})
	var canceled atomic.Bool
	stream := &textServiceStream{applicationHalfClose: application, finished: finished,
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
