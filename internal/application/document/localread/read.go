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
	stage, err := openPresentationStage()
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, stage.Close()) }()
	stream, err := endpointclient.Dial(ctx, socket, endpointstream.Request{Destination: endpointstream.TargetLink, Value: link})
	if err != nil {
		return err
	}
	if err := document.ReceiveFile(ctx, stream, stage); err != nil {
		return err
	}
	return presentStaged(ctx, output, stage)
}
