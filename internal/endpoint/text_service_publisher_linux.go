//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// serveOperation consumes the already reserved worker operation. Startup may
// reserve it before publication so readiness never races another worker use.
func (worker *qualifiedTextWorker) serveOperation(ctx, bounded context.Context, finish func(), produce func(context.Context, chan<- connection.Stream) error) error {
	forwarding, cancel := context.WithCancel(bounded)
	delivered := make(chan connection.Stream)
	forwarded := make(chan error, 1)
	go func() {
		defer close(delivered)
		err := produce(forwarding, delivered)
		if err != nil {
			cancel()
		}
		forwarded <- err
	}()
	operationErr := textdocument.ServeWorkerConnections(forwarding, worker.lifetime.attachment, delivered)
	cancel()
	forwardErr := <-forwarded
	finish()
	callerErr := ctx.Err()
	resultErr := errors.Join(operationErr, forwardErr, callerErr, worker.Close())
	if resultErr == nil && !worker.completedCurrent() {
		resultErr = errors.New("text Publisher completion belongs to a retired context or replaced job")
	}
	return resultErr
}
