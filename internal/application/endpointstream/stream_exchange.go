package endpointstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

// ExchangeStream joins both directions and requires an authenticated terminal
// outcome. It owns input, output and stream through interruption and cleanup.
func ExchangeStream(ctx context.Context, stream Stream, input io.ReadCloser, output io.WriteCloser) error {
	transfer, cancel := context.WithCancel(ctx)
	defer cancel()
	closeInput := sync.OnceValue(input.Close)
	closeStream := sync.OnceValue(stream.Close)
	sent, received := make(chan error, 1), make(chan error, 1)
	return withStreamIO(transfer, func() error {
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
				case outcome.Class != CleanClose:
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

// withStreamIO joins cancellation-driven closure before returning.
func withStreamIO(ctx context.Context, closeIO, exchange func() error) (outcome error) {
	var closeErr error
	closed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(closed)
		closeErr = closeIO()
	})
	defer func() {
		if stop() {
			closeErr = closeIO()
		} else {
			<-closed
		}
		outcome = errors.Join(outcome, closeErr, ctx.Err())
	}()
	return exchange()
}
