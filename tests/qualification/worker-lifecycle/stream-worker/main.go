package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "qualification worker unavailable")
		os.Exit(2)
	}
}

// run owns only the two static systemd worker entrypoints. It does not accept
// an address, Target, executable, profile, authority, Grant or environment
// supplied work shape; the installed Endpoint sends the fixed INIT record.
func run(arguments []string, input io.Reader, output io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("qualification worker command is invalid")
	}
	role := streamqualification.ReaderRole
	switch arguments[0] {
	case "worker-reader":
	case "worker-publisher":
		role = streamqualification.PublisherRole
	default:
		return errors.New("qualification worker command is invalid")
	}
	init, err := streamqualification.ReadInit(input, role)
	if err != nil {
		return err
	}
	if err := streamqualification.WriteReady(output, init.Nonce); err != nil {
		return err
	}
	return streamqualification.RunWorker(context.Background(), &streamReadWriter{Reader: input, Writer: output}, init)
}

type streamReadWriter struct {
	once sync.Once
	err  error
	io.Reader
	io.Writer
}

// Closing inherited descriptors interrupts both directions and lets the worker
// join its reader. In-memory bounded command fixtures have no OS descriptor.
func (stream *streamReadWriter) Close() error {
	stream.once.Do(func() {
		var outcome error
		if closer, ok := stream.Reader.(io.Closer); ok {
			outcome = errors.Join(outcome, closer.Close())
		}
		same := stream.Reader != nil && reflect.TypeOf(stream.Reader).Comparable() && any(stream.Reader) == any(stream.Writer)
		if closer, ok := stream.Writer.(io.Closer); ok && !same {
			outcome = errors.Join(outcome, closer.Close())
		}
		stream.err = outcome
	})
	return stream.err
}
