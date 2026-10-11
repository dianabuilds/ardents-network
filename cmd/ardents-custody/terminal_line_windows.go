package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

// readTerminalLine cancels the actual synchronous console read on its retained
// OS thread and joins it. The thread stays locked until the interrupter retires,
// so cancellation cannot target a replacement reader or another goroutine.
func readTerminalLine(ctx context.Context, file *os.File, secret bool, ready func()) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	interrupt := windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")
	if err := interrupt.Find(); err != nil {
		return nil, err
	}
	type startup struct {
		thread windows.Handle
		err    error
	}
	type outcome struct {
		value []byte
		err   error
	}
	started, completed := make(chan startup), make(chan outcome, 1)
	release, joined := make(chan struct{}), make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(joined)
		thread, err := windows.OpenThread(windows.THREAD_TERMINATE, false, windows.GetCurrentThreadId())
		started <- startup{thread, err}
		if err != nil {
			return
		}
		var value []byte
		if secret {
			value, err = readWindowsSecret(file, ready)
		} else {
			if ready != nil {
				ready()
			}
			value, err = bufio.NewReader(io.LimitReader(file, 1025)).ReadBytes('\n')
		}
		completed <- outcome{value, err}
		<-release
	}()
	start := <-started
	if start.err != nil {
		<-joined
		return nil, start.err
	}
	var result outcome
	var cancelErr error
	select {
	case result = <-completed:
	case <-ctx.Done():
		ticker := time.NewTicker(10 * time.Millisecond)
		for {
			ok, _, err := interrupt.Call(uintptr(start.thread))
			if ok == 0 && !errors.Is(err, windows.ERROR_NOT_FOUND) {
				cancelErr = errors.Join(cancelErr, err)
			}
			select {
			case result = <-completed:
				ticker.Stop()
				goto finished
			case <-ticker.C:
			}
		}
	}
finished:
	closeErr := windows.CloseHandle(start.thread)
	close(release)
	<-joined
	err := errors.Join(result.err, ctx.Err(), cancelErr, closeErr)
	if len(result.value) > 1024 {
		err = errors.Join(err, errors.New("custody terminal input exceeds bound"))
	}
	if err != nil {
		clear(result.value)
		return nil, err
	}
	return result.value, nil
}

// readWindowsSecret reads finite bytes with echo and processed Ctrl+C disabled.
// The containing original OS-thread owner supplies cancellation and join.
func readWindowsSecret(file *os.File, ready func()) (value []byte, resultErr error) {
	defer func() {
		if resultErr != nil {
			clear(value)
			value = nil
		}
	}()
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(file.Fd()), &mode); err != nil {
		return nil, err
	}
	if err := windows.SetConsoleMode(windows.Handle(file.Fd()), mode&^(windows.ENABLE_ECHO_INPUT|windows.ENABLE_LINE_INPUT|windows.ENABLE_PROCESSED_INPUT)); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, windows.SetConsoleMode(windows.Handle(file.Fd()), mode)) }()
	if ready != nil {
		ready()
	}
	var one [1]byte
	defer clear(one[:])
	for {
		read, err := file.Read(one[:])
		if err != nil {
			return value, err
		}
		if read == 0 {
			return value, io.EOF
		}
		switch one[0] {
		case 3:
			return value, context.Canceled
		case '\n':
			continue
		case '\r':
			return value, nil
		case 8, 127:
			if len(value) > 0 {
				value[len(value)-1] = 0
				value = value[:len(value)-1]
			}
			continue
		}
		if len(value) >= 1024 {
			return value, errors.New("custody terminal input exceeds bound")
		}
		value = append(value, one[0])
	}
}
