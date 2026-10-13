package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// NextStream consumes one initial authentication and retains its original
// Source/Responder replacement scope before handing the stream to Execution.
// Publication retains physical retirement even after the Application completes.
func (p *Publisher) NextStream(ctx context.Context, contract executionruntime.StreamContract) (_ *connection.Stream, result error) {
	if p == nil || p.self != p || ctx == nil {
		return nil, errors.New("Publisher stream caller is absent")
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, p.stoppedAcquisition(ctx)
	}
	p.flights.Add(1)
	p.mu.Unlock()
	defer p.flights.Done()
	owner, err := p.NextAuthentication(ctx)
	if err != nil {
		return nil, err
	}
	stream, err := owner.OpenPublisherStream(contract, p.sourceScope, p.responderScope)
	if err != nil {
		cleanup := owner.Close()
		p.config.Operation.RetainCleanup(cleanup)
		return nil, errors.Join(err, cleanup)
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, stream.Close())
			p.config.Operation.RetainCleanup(stream.JoinedCleanup())
		}
	}()
	logical, err := stream.PublisherContext()
	if err != nil {
		return nil, err
	}
	if err := p.check(ctx); err != nil {
		return nil, err
	}
	if err := p.responderScope.Commit(func(checkOriginal func() error) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.closed || p.connections[logical] != nil {
			return errors.New("Publisher logical stream is retired or already retained")
		}
		if err := errors.Join(ctx.Err(), p.ctx.Err(), stream.Context().Err(), p.lifetime.Context().Err(), p.config.Operation.CheckPublisher(), checkOriginal()); err != nil {
			return err
		}
		p.connections[logical] = stream
		p.connectionUsers.Add(1)
		return nil
	}); err != nil {
		return nil, err
	}
	go p.watchConnection(logical, stream)
	return stream, nil
}

// stoppedAcquisition identifies only this owner's idle acquisition retirement.
// It is not a stream EOF or a successful withdrawal/physical cleanup result.
type stoppedAcquisition struct{ owner *Publisher }

func (*stoppedAcquisition) Error() string { return "Publisher stopped idle stream acquisition" }

func (p *Publisher) stoppedAcquisition(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return &stoppedAcquisition{owner: p}
	}
	return p.ctx.Err()
}

// StreamAcquisitionStopped recognizes only the original owner's direct idle
// stop result. Callers must still join Close/Withdraw and preserve their errors;
// an aggregate containing another failure is never classified as an idle stop.
func (p *Publisher) StreamAcquisitionStopped(err error) bool {
	stopped, ok := err.(*stoppedAcquisition)
	return p != nil && p.self == p && ok && stopped.owner == p
}

// The full canonical tuple locates retained work; it cannot authenticate it.
// BindPublisherRecipient independently matches the live stream and private
// proof, and original-key Continuity still precedes every generation commit.
func (p *Publisher) findConnection(request capsule.Request, digest [32]byte) (*connection.Stream, error) {
	value := p.credential.Delegation()
	identity := wire.LogicalIdentity{Network: request.Network, Target: request.Target, Instance: value.Instance,
		InstanceGeneration: value.Generation, Publication: request.PublicationDigest, Profile: request.ProfileDigest,
		ConnectionNonce: request.ConnectionNonce, InitiatorBinding: request.InitiatorBinding,
		WorkSafetyNotAfter: request.WorkSafetyNotAfter, WorkSafetyMaximum: request.WorkSafetyMaximum, NoNewRecoveryAfter: request.NoNewRecoveryAfter}
	logical, _, err := wire.Contexts(identity, digest, request.AttachmentGeneration)
	if err != nil || request.AttachmentGeneration <= 1 {
		return nil, errors.Join(errors.New("Publisher replacement tuple is invalid"), err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	stream := p.connections[logical]
	if p.closed || stream == nil {
		return nil, errors.New("Publisher original logical stream is unavailable")
	}
	return stream, nil
}

func (p *Publisher) dispatchReplacement(state *initialDelivery) {
	defer state.close()
	if err := state.observe(); err != nil {
		return
	}
	request, _, err := state.opening.Candidate(state.ctx)
	if err != nil {
		return
	}
	proposal, err := state.opening.stream.PreparePublisherReplacement(state.ctx, state.opening.private, join.JoinConfig{
		Duty: state.opening.rendezvous, Secret: request.JoinSecret, Context: request.HandshakeContext,
		Deadline: time.Unix(request.WorkSafetyNotAfter, 0).UTC(), SetupDeadline: request.Deadline})
	if err != nil {
		return // Connection already joins and retains actual rejected cleanup.
	}
	defer proposal.Close()
	_ = proposal.Commit()
}

func (p *Publisher) watchConnection(logical [32]byte, stream *connection.Stream) {
	defer p.connectionUsers.Done()
	<-stream.Joined()
	_ = stream.Close()
	cleanup := stream.JoinedCleanup()
	p.mu.Lock()
	p.failure = errors.Join(p.failure, cleanup)
	p.cleanupErr = errors.Join(p.cleanupErr, cleanup)
	if p.connections[logical] == stream {
		delete(p.connections, logical)
	}
	p.mu.Unlock()
}

func (p *Publisher) joinConnections() {
	p.mu.Lock()
	streams := make([]*connection.Stream, 0, len(p.connections))
	for _, stream := range p.connections {
		streams = append(streams, stream)
	}
	p.mu.Unlock()
	for _, stream := range streams {
		_ = stream.Close()
	}
	p.connectionUsers.Wait()
}
