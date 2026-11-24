package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

func runStreamCommand(ctx context.Context, args []string, diagnostic io.Writer) int {
	publish := len(args) == 2 && args[0] == "publish" && filepath.IsAbs(args[1])
	connect := len(args) == 4 && args[0] == "connect" && filepath.IsAbs(args[1]) && args[2] == "--link-file" && filepath.IsAbs(args[3])
	if !publish && !connect {
		_, err := io.WriteString(diagnostic, "usage: ardents stream connect APPLICATION_SOCKET --link-file PRIVATE_LINK_FILE\n"+
			"       ardents stream publish ADMINISTRATION_SOCKET\n")
		return streamCommandStatus(2, err)
	}
	if runtime.GOOS != "linux" {
		_, err := fmt.Fprintln(diagnostic, "Application stream failed: application stream requires the selected Linux host")
		return streamCommandStatus(1, err)
	}
	var err error
	if publish {
		err = publishApplicationStream(ctx, args[1])
	} else {
		err = connectApplicationStream(ctx, args[1], args[3])
	}
	if err != nil {
		_, writeErr := fmt.Fprintln(diagnostic, "Application stream failed:", err)
		return streamCommandStatus(1, writeErr)
	}
	return 0
}

func streamCommandStatus(code int, diagnosticErr error) int {
	if diagnosticErr != nil {
		return 2
	}
	return code
}

func connectApplicationStream(ctx context.Context, socket, linkFile string) error {
	ctx, cancel := context.WithTimeout(ctx, connection.ReaderTimeout)
	defer cancel()
	link, err := readPrivateStreamLink(linkFile)
	if err != nil {
		return err
	}
	return runConsoleStream(ctx, "local stream admission failed", func() (connection.Client, error) {
		return connection.Dial(ctx, socket, connection.Request{
			Application: connection.ByteStream, Destination: connection.TargetLink, Value: link,
		})
	})
}

// runConsoleStream owns the descriptors until a successful dial transfers
// them to the exchange, which also joins their cancellation cleanup.
func runConsoleStream(ctx context.Context, dialFailure string, dial func() (connection.Client, error)) (outcome error) {
	input, output, err := openDocumentCommandIO()
	if err != nil {
		// Descriptor errors may include local filenames in their messages.
		return errors.New("local stream input or output is unavailable")
	}
	handedOff := false
	defer func() {
		if !handedOff {
			outcome = errors.Join(outcome, input.Close(), output.Close())
		}
	}()
	stream, err := dial()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New(dialFailure)
	}
	handedOff = true
	return exchangeApplicationStream(ctx, stream, input, output)
}

func readPrivateStreamLink(path string) (string, error) {
	unavailable := errors.New("private Target Link file is unavailable")
	input, err := openPrivateLinkFile(path)
	if err != nil {
		return "", unavailable
	}
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		if closeErr := input.Close(); closeErr != nil {
			return "", errors.Join(unavailable, errors.New("private Target Link close failed"))
		}
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
	closeInput := sync.OnceValue(input.Close)
	closeStream := sync.OnceValue(stream.Close)
	sent, received := make(chan error, 1), make(chan error, 1)
	return withCommandIO(transfer, func() error {
		return errors.Join(closeInput(), output.Close(), closeStream())
	}, func() error {
		go func(result chan<- error) {
			_, err := io.Copy(stream, input)
			if err == nil {
				err = stream.CloseInput()
			}
			result <- err
		}(sent)
		go func(result chan<- error) {
			_, err := io.Copy(output, stream)
			result <- err
		}(received)

		terminal, canceled := stream.Done(), transfer.Done()
		var sendErr, receiveErr, terminalErr error
		inputInterrupted := false
		for sent != nil || received != nil || terminal != nil {
			select {
			case err := <-sent:
				sent = nil
				// A verified terminal can interrupt an otherwise idle stdin.
				interrupted := inputInterrupted && (errors.Is(err, os.ErrClosed) ||
					errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed))
				if err != nil && !interrupted {
					sendErr = err
					cancel()
				}
			case receiveErr = <-received:
				received = nil
				if receiveErr != nil {
					cancel()
				}
			case outcome, open := <-terminal:
				terminal = nil
				switch {
				case !open:
					terminalErr = errors.New("authenticated stream did not complete")
				case outcome.Class != connection.CleanClose:
					terminalErr = fmt.Errorf("authenticated stream did not complete (%s): %s", outcome.Class, outcome.Reason)
				}
				if sent != nil {
					inputInterrupted = true
					if err := closeInput(); err != nil {
						cancel()
					}
				}
			case <-canceled:
				canceled = nil
				terminal = nil
			}
			// After stdout drains, closing the terminal stream can also
			// interrupt a sender blocked in the transport instead of stdin.
			if terminal == nil && received == nil && sent != nil && canceled != nil {
				if err := closeStream(); err != nil {
					cancel()
				}
			}
		}
		return errors.Join(sendErr, receiveErr, terminalErr)
	})
}

func publishApplicationStream(ctx context.Context, socket string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	return runConsoleStream(ctx, "local byte-stream publication failed", func() (connection.Client, error) {
		return connection.DialPublishedStream(ctx, socket)
	})
}
