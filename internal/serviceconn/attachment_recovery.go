package serviceconn

import (
	"context"
	"errors"
	"time"
)

const recoveryPublicationReserve = 10 * time.Millisecond

func (stream *recoveryStream) recoverAttachment(failed *securedAttachment) error {
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
	if !stream.authorizationTime().Before(time.Unix(stream.binding.NoNewRecoveryAfter, 0)) {
		stream.mu.Unlock()
		return errors.New("current Credential and Work Safety expired before recovery")
	}
	stream.recovering = true
	if stream.episodeEnd.IsZero() {
		stream.episodeEnd = recoveryEpisodeDeadline(stream.lastProgress, time.Now())
	}
	deadline := recoveryWorkDeadline(stream.episodeEnd)
	safetyRemaining := time.Unix(stream.binding.NoNewRecoveryAfter, 0).Sub(stream.authorizationTime())
	if safetyDeadline := time.Now().Add(safetyRemaining); safetyDeadline.Before(deadline) {
		deadline = safetyDeadline
	}
	generation := failed.generation + 1
	state := continuityState{sendBase: stream.sendBase, sendEnd: stream.sendEnd, recvNext: stream.recvNext}
	stream.mu.Unlock()
	failed.close()

	var last error
	for {
		if !stream.authorizationTime().Before(time.Unix(stream.binding.NoNewRecoveryAfter, 0)) {
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
		request := stream.binding
		request.Generation, request.Deadline, request.NetworkID = generation, deadline, stream.credential.NetworkID
		if stream.client {
			request.Role = "client"
		} else {
			request.Role = "publisher"
		}
		raw, err := stream.opener(attempt, request)
		if err == nil {
			var attachment *securedAttachment
			var fresh [32]byte
			if stream.client {
				attachment, fresh, err = secureClient(attempt, raw, stream.credential, stream.context, generation)
			} else {
				attachment, fresh, err = securePublisher(attempt, raw, stream.credential, stream.private, stream.context, generation)
			}
			erase(fresh[:])
			if errors.Is(err, errInstanceMismatch) {
				err = errActiveViolation
			}
			if err == nil {
				var peer peerContinuity
				peer, err = exchangeContinuityProof(attempt, attachment, stream.continuity,
					stream.credential, stream.binding, stream.client, state)
				if err == nil {
					err = stream.commitAttachment(failed, attachment, peer)
				}
				if err != nil {
					attachment.close()
				}
			}
		}
		cancel()
		releaseTimer()
		if err == nil {
			return nil
		}
		last = err
		if errors.Is(err, errActiveViolation) {
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

func (stream *recoveryStream) commitAttachment(failed, attachment *securedAttachment, peer peerContinuity) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.current != failed || attachment.generation <= failed.generation ||
		peer.recvNext < stream.sendBase || peer.recvNext > stream.sendEnd || peer.sendEnd < stream.recvNext {
		return errActiveViolation
	}
	if peer.peerNonce == [32]byte{} || peer.localNonce == [32]byte{} || peer.peerNonce == peer.localNonce {
		return errActiveViolation
	}
	if err := stream.acknowledgeLocked(peer.recvNext); err != nil {
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
