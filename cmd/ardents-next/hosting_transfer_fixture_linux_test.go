//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// transferLoopback owns both sockets and joins its only child before returning.
// Payload is bounded synthetic local data; no external address is accepted.
func transferLoopback(ctx context.Context, count uint64, deadline time.Time, current func() error) error {
	return transferLocal(ctx, count, deadline, current, discardPayload)
}

func discardPayload(reader io.Reader, count int64) error {
	_, err := io.CopyN(io.Discard, reader, count)
	return err
}

func transferLocal(ctx context.Context, count uint64, deadline time.Time, current func() error, receive func(io.Reader, int64) error) (outcome error) {
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
	closeListener := joinedClose(listener)
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
	closeSender := joinedClose(sender)
	defer func() { outcome = errors.Join(outcome, closeSender()) }()
	receiver, err := listener.Accept()
	if err != nil {
		return err
	}
	closeReceiver := joinedClose(receiver)
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

// joinedClose retains the first cleanup result across cancellation and unwind.
func joinedClose(resource io.Closer) func() error {
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
