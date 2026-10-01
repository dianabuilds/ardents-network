//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	processdiag "github.com/dianabuilds/ardents-network/internal/diagnostics/process"
	"github.com/dianabuilds/ardents-network/internal/endpoint/service"
)

func (worker *qualifiedWorker) completeServiceRead(ctx, bounded context.Context, finish func(), stream *service.Stream, err error) ([]byte, error) {
	var body []byte
	if err == nil {
		exchanged := processdiag.ObserveConnection(bounded, processdiag.DocumentExchange)
		body, err = textdocument.ReadWorkerConnection(bounded, worker.lifetime.attachment, stream)
		exchanged(err)
		closed := processdiag.ObserveConnection(bounded, processdiag.ServiceClose)
		closeErr := stream.Close()
		closed(closeErr)
		err = errors.Join(err, closeErr)
	}
	finish()
	closed := processdiag.ObserveConnection(bounded, processdiag.WorkerClose)
	closeErr := worker.Close()
	closed(closeErr)
	resultErr := errors.Join(err, closeErr, ctx.Err())
	if resultErr == nil {
		current := processdiag.ObserveConnection(bounded, processdiag.CurrentOwner)
		if !worker.completedCurrent() {
			resultErr = errors.New("text Service result belongs to a retired context or replaced job")
		}
		current(resultErr)
	}
	if resultErr != nil {
		clear(body)
		return nil, resultErr
	}
	return body, nil
}
