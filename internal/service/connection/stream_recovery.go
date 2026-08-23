package connection

import (
	"context"
	"errors"
	"time"
)

const recoveryPublicationReserve = 10 * time.Millisecond

func (stream *Stream) recoverAttachment(failed *Attachment) error {
	stream.mu.Lock()
	for stream.recovering && stream.terminal == nil && stream.current == failed {
		stream.cond.Wait()
	}
	if stream.terminal != nil {
		err := stream.terminal
		stream.mu.Unlock()
		return err
	}
	if stream.current != failed {
		stream.mu.Unlock()
		return nil
	}
	if stream.opener == nil {
		stream.mu.Unlock()
		return errors.New("no safe eligible Route Attachment remains")
	}
	if !stream.authorizationTime().Before(time.Unix(stream.recovery.NoNewRecoveryAfter, 0)) {
		stream.mu.Unlock()
		return errors.New("current Credential and Work Safety expired before recovery")
	}
	stream.recovering = true
	if stream.episodeEnd.IsZero() {
		stream.episodeEnd = recoveryEpisodeDeadline(stream.lastProgress, time.Now())
	}
	deadline := recoveryWorkDeadline(stream.episodeEnd)
	safetyRemaining := time.Unix(stream.recovery.NoNewRecoveryAfter, 0).Sub(stream.authorizationTime())
	if safetyDeadline := time.Now().Add(safetyRemaining); safetyDeadline.Before(deadline) {
		deadline = safetyDeadline
	}
	generation := failed.generation + 1
	state := ContinuityExchange{Key: stream.continuity, Generation: generation, SendBase: stream.sendBase,
		SendEnd: stream.sendEnd, ReceiveNext: stream.recvNext}
	stream.mu.Unlock()
	failed.closeCarrier()

	var last error
	for {
		if !stream.authorizationTime().Before(time.Unix(stream.recovery.NoNewRecoveryAfter, 0)) {
			last = errors.New("current Credential and Work Safety expired during recovery")
			break
		}
		stream.mu.Lock()
		if stream.proposals >= proposalLimit || time.Now().After(deadline) {
			stream.mu.Unlock()
			break
		}
		stream.proposals++
		stream.mu.Unlock()
		releaseTimer := acquireResource(stream.resources, "timer")
		attempt, cancel := context.WithDeadline(stream.ctx, deadline)
		request := stream.recovery
		request.Generation, request.Deadline, request.NetworkID = generation, deadline, stream.networkID
		if stream.client {
			request.Role = "client"
		} else {
			request.Role = "publisher"
		}
		attachment, err := stream.opener(attempt, request)
		if err == nil {
			state.Role = RoleClient
			if !stream.client {
				state.Role = RolePublisher
			}
			state.Context, state.ExporterCommitment = attachment.context, attachment.exporterCommitment
			peer, exchangeErr := ExchangeContinuity(attempt, attachment.carrier, state)
			if exchangeErr != nil {
				err = ErrActiveViolation
			} else {
				err = stream.commitAttachment(failed, attachment, peer)
			}
			if err != nil {
				attachment.closeCarrier()
			}
		}
		cancel()
		releaseTimer()
		if err == nil {
			return nil
		}
		last = err
		if errors.Is(err, ErrActiveViolation) {
			break
		}
	}
	if last == nil {
		last = errors.New("route Attachment proposal limit or recovery deadline reached")
	}
	stream.fail(last)
	return last
}

func recoveryEpisodeDeadline(lastProgress, detected time.Time) time.Time {
	return recoveryEpisodeStart(lastProgress, detected).Add(recoveryLimit)
}

func recoveryWorkDeadline(episodeEnd time.Time) time.Time {
	return episodeEnd.Add(-recoveryPublicationReserve)
}

func recoveryEpisodeStart(lastProgress, detected time.Time) time.Time {
	if lastProgress.IsZero() || lastProgress.After(detected) {
		lastProgress = detected
	}
	return lastProgress
}

func (stream *Stream) commitAttachment(failed, attachment *Attachment, peer ContinuityPeer) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.current != failed || attachment.generation <= failed.generation ||
		peer.ReceiveNext < stream.sendBase || peer.ReceiveNext > stream.sendEnd || peer.SendEnd < stream.recvNext {
		return ErrActiveViolation
	}
	if peer.PeerNonce == [32]byte{} || peer.LocalNonce == [32]byte{} || peer.PeerNonce == peer.LocalNonce {
		return ErrActiveViolation
	}
	if err := stream.acknowledgeLocked(peer.ReceiveNext); err != nil {
		return err
	}
	stream.current = attachment
	stream.sendNext = stream.sendBase
	stream.recoveries++
	stream.recovering = false
	stream.proposals = 0
	stream.episodeEnd = time.Time{}
	stream.cond.Broadcast()
	return nil
}
