//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

func (worker *qualifiedTextWorker) completeServiceRead(ctx, bounded context.Context, finish func(), stream *textServiceStream, err error) ([]byte, error) {
	var body []byte
	if err == nil {
		body, err = textdocument.ReadWorkerConnection(bounded, worker.lifetime.attachment, stream)
		err = errors.Join(err, stream.Close())
	}
	finish()
	resultErr := errors.Join(err, worker.Close(), ctx.Err())
	if resultErr == nil && !worker.completedCurrent() {
		resultErr = errors.New("text Service result belongs to a retired context or replaced job")
	}
	if resultErr != nil {
		clear(body)
		return nil, resultErr
	}
	return body, nil
}
