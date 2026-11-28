package connection

import (
	"sync"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
)

// ReaderContinuation permits one freshly admitted Source to use the already
// joined transition under the same qualified Reader operation.
type ReaderContinuation struct {
	self     *ReaderContinuation
	previous *Stream
	once     sync.Once
}

// ClaimReaderContinuation takes the joined transition before downstream Route
// preparation. It neither claims another local job nor grants network spend.
func (stream *Stream) ClaimReaderContinuation(operation *executionruntime.Operation) (*ReaderContinuation, error) {
	if stream == nil || stream.self != stream || stream.owner == nil ||
		operation == nil || operation != stream.owner.operation || stream.owner.role != wire.RoleInitiator {
		return nil, errContinuationRefused
	}
	if err := operation.CheckReader(); err != nil {
		return nil, err
	}
	select {
	case <-stream.watcherDone:
	default:
		return nil, errContinuationRefused
	}
	if stream.JoinedCleanup() != nil || stream.closeErr != nil || stream.segmentBoundary == nil {
		return nil, errContinuationRefused
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.readerContinuationClaimed {
		return nil, errContinuationRefused
	}
	stream.readerContinuationClaimed = true
	claim := &ReaderContinuation{previous: stream}
	claim.self = claim
	return claim, nil
}

func (claim *ReaderContinuation) consume(config InitiatorSourceConfig, target [32]byte) (*Stream, error) {
	if claim.self != claim || claim.previous == nil {
		return nil, errContinuationRefused
	}
	previous := claim.previous
	identity := previous.owner.identity
	if config.Operation != previous.owner.operation || config.Network != identity.Network || config.Profile != identity.Profile || target != identity.Target {
		return nil, errContinuationRefused
	}
	var stream *Stream
	claim.once.Do(func() { stream = previous })
	if stream == nil {
		return nil, errContinuationRefused
	}
	return stream, nil
}
