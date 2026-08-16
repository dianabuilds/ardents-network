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
	stop := context.AfterFunc(ctx, func() { _ = pipe.Close() })
	raw, err := io.ReadAll(io.LimitReader(pipe, maximum+1))
	stopped := stop()
	if !stopped && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil || int64(len(raw)) > maximum {
		return nil, errors.New("inherited control frame is invalid")
	}
	return raw, nil
}
