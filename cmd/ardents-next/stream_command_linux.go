//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

func connectApplicationStream(ctx context.Context, socket, linkFile string) error {
	ctx, cancel := context.WithTimeout(ctx, connection.ReaderTimeout)
	defer cancel()
	link, err := readPrivateStreamLink(linkFile)
	if err != nil {
		return err
	}
	input, output, err := openDocumentCommandIO()
	if err != nil {
		return errors.New("local stream input or output is unavailable")
	}
	stream, err := connection.Dial(ctx, socket, connection.Request{
		Application: connection.ByteStream, Destination: connection.TargetLink, Value: link,
	})
	if err != nil {
		return errors.Join(errors.New("local stream admission failed"), input.Close(), output.Close())
	}
	return exchangeApplicationStream(ctx, stream, input, output)
}

func readPrivateStreamLink(path string) (string, error) {
	unavailable := errors.New("private Target Link file is unavailable")
	input, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", unavailable
	}
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		_ = input.Close()
		return "", unavailable
	}
	link, err := readDocumentDestination(input)
	if errors.Join(err, input.Close()) != nil {
		return "", unavailable
	}
	return link, nil
}

func exchangeApplicationStream(ctx context.Context, stream connection.Stream, input io.ReadCloser, output io.WriteCloser) error {
	transfer, cancel := context.WithCancel(ctx)
	defer cancel()
	joined := make(chan error, 2)
	return withCommandIO(transfer, func() error {
		return errors.Join(input.Close(), output.Close(), stream.Close())
	}, func() error {
		go func() {
			_, err := io.Copy(stream, input)
			if err == nil {
				err = stream.CloseInput()
			}
			joined <- err
		}()
		go func() {
			_, err := io.Copy(output, stream)
			joined <- err
		}()
		first := <-joined
		if first != nil {
			cancel()
		}
		if err := errors.Join(first, <-joined); err != nil {
			return err
		}
		select {
		case outcome, open := <-stream.Done():
			if !open || outcome.Class != connection.CleanClose {
				return errors.New("authenticated stream did not complete")
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

func publishApplicationStream(ctx context.Context, socket string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	input, output, err := openDocumentCommandIO()
	if err != nil {
		return errors.New("local stream input or output is unavailable")
	}
	stream, err := connection.DialPublishedStream(ctx, socket)
	if err != nil {
		return errors.Join(errors.New("local byte-stream publication failed"), input.Close(), output.Close())
	}
	return exchangeApplicationStream(ctx, stream, input, output)
}
