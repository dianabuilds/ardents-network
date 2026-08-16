package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
)

func openInheritedPipe(encoded string) (*os.File, error) {
	handle, err := strconv.ParseUint(encoded, 10, 64)
	if err != nil || handle < 3 {
		return nil, errors.New("inherited control handle is invalid")
	}
	pipe := os.NewFile(uintptr(handle), "ardents-inherited-control")
	if pipe == nil {
		return nil, errors.New("inherited control handle is invalid")
	}
	info, err := pipe.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		_ = pipe.Close()
		return nil, errors.New("inherited control handle is not a pipe")
	}
	return pipe, nil
}

func readInheritedPipe(ctx context.Context, pipe *os.File, maximum int64) ([]byte, error) {
	type result struct {
		raw []byte
		err error
	}
	read := make(chan result, 1)
	go func() {
		raw, err := io.ReadAll(io.LimitReader(pipe, maximum+1))
		read <- result{raw: raw, err: err}
	}()
	select {
	case value := <-read:
		if value.err != nil || int64(len(value.raw)) > maximum {
			return nil, errors.New("inherited control frame is invalid")
		}
		return value.raw, nil
	case <-ctx.Done():
		_ = pipe.Close()
		return nil, ctx.Err()
	}
}
