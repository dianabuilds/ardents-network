//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func networkDiscardPayload(reader io.Reader, count int64) error {
	_, err := io.CopyN(io.Discard, reader, count)
	return err
}

func networkTransferLocal(ctx context.Context, count uint64, deadline time.Time, current func() error, receive func(io.Reader, int64) error) (outcome error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := current(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	closeListener := networkJoinedClose(listener)
	stopped := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = closeListener(); close(stopped) })
	defer func() {
		if !stop() {
			<-stopped
		}
	}()
	defer func() { outcome = errors.Join(outcome, closeListener()) }()
	sender, err := (&net.Dialer{}).DialContext(ctx, "tcp4", listener.Addr().String())
	if err != nil {
		return err
	}
	closeSender := networkJoinedClose(sender)
	defer func() { outcome = errors.Join(outcome, closeSender()) }()
	receiver, err := listener.Accept()
	if err != nil {
		return err
	}
	closeReceiver := networkJoinedClose(receiver)
	defer func() { outcome = errors.Join(outcome, closeReceiver()) }()
	if err = errors.Join(sender.SetDeadline(deadline), receiver.SetDeadline(deadline)); err != nil {
		return err
	}
	joinedCancellation := make(chan struct{})
	stopWork := context.AfterFunc(ctx, func() { _ = closeSender(); _ = closeReceiver(); close(joinedCancellation) })
	defer func() {
		if !stopWork() {
			<-joinedCancellation
		}
	}()
	readDone := make(chan error, 1)
	go func() { readDone <- receive(receiver, int64(count)) }()
	buffer := make([]byte, 4096)
	for left := count; left > 0; {
		if outcome = current(); outcome != nil {
			break
		}
		n := min(left, uint64(len(buffer)))
		written, err := sender.Write(buffer[:n])
		if err != nil {
			outcome = err
			break
		}
		if written != int(n) {
			outcome = io.ErrShortWrite
			break
		}
		left -= n
	}
	if outcome != nil {
		_ = closeSender()
		_ = closeReceiver()
	}
	outcome = errors.Join(outcome, <-readDone, ctx.Err())
	if outcome == nil {
		outcome = current()
	}
	return outcome
}

// networkJoinedClose retains the first cleanup result across cancellation and unwind.
func networkJoinedClose(resource io.Closer) func() error {
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			result = resource.Close()
			if errors.Is(result, net.ErrClosed) {
				result = nil
			}
		})
		return result
	}
}

// networkWorkAuthority retains the accepted facts and a monotonic observation floor.
func networkWorkAuthority(initial admission.AuthorityFacts, floor, deadline time.Time, observe func() (admission.AuthorityFacts, time.Time, error)) func() error {
	return func() error {
		facts, now, err := observe()
		if err != nil || facts != initial || now.Before(floor) || now.Before(initial.NotBefore) || !now.Before(initial.NotAfter) || !now.Before(deadline) {
			return errors.Join(errors.New("work authority unavailable"), err)
		}
		floor = now
		return nil
	}
}
