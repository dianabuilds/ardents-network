package localread

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	endpointclient "github.com/dianabuilds/ardents-network/internal/application/endpointstream/client"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream/targetlink"
)

// Run owns the input and output until the single document read and any
// cancellation-driven closure have joined.
func Run(ctx context.Context, socket string, input io.ReadCloser, output io.WriteCloser) (outcome error) {
	if ctx == nil || !filepath.IsAbs(socket) || input == nil || output == nil {
		return endpointstream.ErrInvalidLocalInput
	}
	var closeErr error
	closed := make(chan struct{})
	closeIO := func() error { return errors.Join(input.Close(), output.Close()) }
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
	if err := ctx.Err(); err != nil {
		return err
	}
	link, err := targetlink.Read(input)
	if err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, endpointstream.ReaderTimeout)
	defer cancel()
	stream, err := endpointclient.Dial(bounded, socket, endpointstream.Request{Destination: endpointstream.TargetLink, Value: link})
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	body, err := document.Read(bounded, stream)
	defer clear(body)
	if err != nil {
		return errors.Join(err, bounded.Err())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return writePlainText(output, body)
}
