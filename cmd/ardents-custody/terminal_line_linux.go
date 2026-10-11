package main

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// readTerminalLine keeps one original terminal reader. Polling bounds only
// cancellation observation; it grants no second password attempt or deadline.
func readTerminalLine(ctx context.Context, file *os.File, secret bool, ready func()) (value []byte, resultErr error) {
	fd := int(file.Fd())
	defer func() {
		if resultErr != nil {
			clear(value)
			value = nil
		}
	}()
	if secret {
		state, err := term.MakeRaw(fd)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, term.Restore(fd, state)) }()
	}
	if ready != nil {
		ready()
	}
	var one [1]byte
	defer clear(one[:])
	for {
		if err := ctx.Err(); err != nil {
			return value, err
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, 20)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return value, err
		}
		if ready == 0 {
			continue
		}
		n, err := unix.Read(fd, one[:])
		if err != nil {
			return value, err
		}
		if n == 0 {
			return value, io.EOF
		}
		switch one[0] {
		case '\r', '\n':
			return value, ctx.Err()
		case 3:
			return value, context.Canceled
		case 8, 127:
			if secret && len(value) > 0 {
				value[len(value)-1] = 0
				value = value[:len(value)-1]
				continue
			}
		}
		if len(value) >= 1024 {
			return value, errors.New("custody terminal input exceeds bound")
		}
		value = append(value, one[0])
	}
}
