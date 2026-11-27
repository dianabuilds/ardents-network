package client

import (
	"context"
	"errors"
	"time"

	appconsole "github.com/dianabuilds/ardents-network/internal/application/console"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream/targetlink"
)

// ConnectLocalStream opens the private destination and joins the local exchange.
func ConnectLocalStream(ctx context.Context, socket, linkFile string) error {
	if err := checkHost(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, endpointstream.ReaderTimeout)
	defer cancel()
	link, err := targetlink.ReadPrivate(linkFile)
	if err != nil {
		return err
	}
	return runConsoleStream(ctx, "local stream admission failed", func() (endpointstream.Stream, error) {
		return Dial(ctx, socket, endpointstream.Request{
			Destination: endpointstream.TargetLink, Value: link,
		})
	})
}

// runConsoleStream owns the descriptors until a successful dial transfers
// them to the exchange, which also joins their cancellation cleanup.
func runConsoleStream(ctx context.Context, dialFailure string, dial func() (endpointstream.Stream, error)) (outcome error) {
	input, output, err := appconsole.Open()
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
	return exchangeStream(ctx, stream, input, output)
}

// PublishLocalStream joins the locally published byte-stream exchange.
func PublishLocalStream(ctx context.Context, socket string) error {
	if err := checkHost(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	return runConsoleStream(ctx, "local byte-stream publication failed", func() (endpointstream.Stream, error) {
		return DialPublishedStream(ctx, socket)
	})
}
