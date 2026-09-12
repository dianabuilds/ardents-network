//go:build linux

package endpoint

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
	"net"
)

// Component fixtures compose already-qualified worker operations directly.
// They do not exercise installed startup, publication readiness or AAI3 admission.
// Production uses startPublication and textConnection.Open with reserved operations.
// serveNetwork retains one Publisher worker across independent network reads.
// Introduction acceptance, JOIN and Service authentication are Endpoint-owned;
// the Application sees only streams authenticated for this exact live job.
// Cancellation terminates and joins the producer, bridge and installed worker.
func (worker *qualifiedTextWorker) serveNetwork(ctx context.Context) error {
	return worker.serveFrom(ctx, worker.produceNetwork)
}

// serve accepts only this Publisher job's opaque authenticated streams.
// Closing incoming requests a finite drain. Cancellation joins the bridge and
// any stream received but not yet transferred before installed worker cleanup.
func (worker *qualifiedTextWorker) serve(ctx context.Context, incoming <-chan *textServiceStream) error {
	if incoming == nil {
		return errors.New("text Publisher Connections unavailable")
	}
	return worker.serveFrom(ctx, func(forwarding context.Context, delivered chan<- connection.Stream) error {
		return worker.forwardServiceStreams(forwarding, incoming, delivered)
	})
}

// serveFrom joins the producer before retiring the worker operation. A producer
// owns every stream until the bridge accepts it; no admission queue is retained.
func (worker *qualifiedTextWorker) serveFrom(ctx context.Context, produce func(context.Context, chan<- connection.Stream) error) error {
	bounded, finish, err := worker.beginOperation(ctx, broker.Administration)
	if err != nil {
		return err
	}
	return worker.serveOperation(ctx, bounded, finish, produce)
}

// There is no buffered admission queue. A received stream remains owned here
// until the worker bridge receives it; all other inputs remain producer-owned.
func (worker *qualifiedTextWorker) forwardServiceStreams(ctx context.Context, incoming <-chan *textServiceStream,
	delivered chan<- connection.Stream) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case stream, ok := <-incoming:
			if !ok {
				return nil
			}
			if stream == nil || stream.binding == nil || stream.binding.job != worker.job ||
				stream.binding.owner != worker.job.owner {
				return errors.Join(errors.New("text Publisher Connection belongs to another job"), stream.Close())
			}
			if err := errors.Join(ctx.Err(), stream.binding.current()); err != nil {
				return errors.Join(err, stream.Close())
			}
			select {
			case delivered <- stream:
			case <-ctx.Done():
				return errors.Join(ctx.Err(), stream.Close())
			}
		}
	}
}

// readService holds the qualified worker's operation from Service setup
// through joined stream I/O, then joins the worker before exposing a result.
// The Route/capsule producer must use this exact binding and authorized job.
func (worker *qualifiedTextWorker) readService(ctx context.Context, binding *textServiceBinding, raw net.Conn, capsuleDigest [32]byte) ([]byte, error) {
	if worker == nil || binding == nil || worker.job == nil || worker.job != binding.job || worker.job.owner != binding.owner {
		return nil, errors.Join(errors.New("text Service stream belongs to a different worker"), closeTextServiceInput(raw))
	}
	bounded, finish, err := worker.beginOperation(ctx, broker.Connection)
	if err != nil {
		return nil, errors.Join(err, closeTextServiceInput(raw))
	}
	stream, err := binding.openTextServiceStream(bounded, raw, capsuleDigest)
	return worker.completeServiceRead(ctx, bounded, finish, stream, err)
}

func closeTextServiceInput(raw net.Conn) error {
	if raw == nil {
		return nil
	}
	return raw.Close()
}

// readTarget begins verified worker use before any destination-dependent lookup,
// issuance or JOIN. Endpoint alone resolves and authenticates the Service; the
// worker receives only that authorized stream, then the complete tree is joined.
func (worker *qualifiedTextWorker) readTarget(ctx context.Context, destination targetlink.Link, bounds [3]int64) ([]byte, error) {
	bounded, finish, err := worker.beginOperation(ctx, broker.Connection)
	if err != nil {
		return nil, err
	}
	owner := worker.job.owner
	attempt, err := owner.prepareTextIntroduction(bounded, worker.job, destination, bounds)
	var stream *textServiceStream
	if err == nil {
		stream, err = owner.openTextJoinedService(bounded, worker.job, attempt)
	}
	return worker.completeServiceRead(ctx, bounded, finish, stream, err)
}
