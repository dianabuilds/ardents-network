package connection

import (
	"errors"
	"time"
)

const (
	recoveryProposalLimit = 3
	recoveryEpisodeLimit  = 15 * time.Second
	recoveryJoinReserve   = 10 * time.Millisecond
)

var errRecoveryExhausted = errors.New("service Connection recovery bound reached")

// recoveryEpisode retains the finite proposal budget under the stream lock.
// It is a scheduling mechanism, not an opener or authorization. The stream
// must check the original operation, Network and signed bounds around every
// effect, authenticate Continuity, and join rejected physical work. A ticket
// neither proves that work joined nor grants a replacement Attachment.
type recoveryEpisode struct {
	generation   uint64
	lastProgress time.Time
	cutoff       time.Time
	deadline     time.Time
	proposals    int
	active       *recoveryProposal
	stopped      bool
}

type recoveryProposal struct {
	generation uint64
	deadline   time.Time
	consumed   bool
}

// cutoff is the already admitted local deadline for original no-new-recovery
// authority. It may not be renewed when an Attachment or episode changes.
func newRecoveryEpisode(generation uint64, started, cutoff time.Time) (*recoveryEpisode, error) {
	if generation == 0 || started.IsZero() || !started.Before(cutoff) {
		return nil, errIntegrity
	}
	return &recoveryEpisode{generation: generation, lastProgress: started, cutoff: cutoff}, nil
}

// progress records actual authenticated byte/control progress, never an open
// attempt or physical retry. Progress cannot extend an already started episode.
func (episode *recoveryEpisode) progress(at time.Time) {
	if !episode.stopped && at.After(episode.lastProgress) {
		episode.lastProgress = at
	}
}

func (episode *recoveryEpisode) begin(failedGeneration uint64, detected time.Time) (*recoveryProposal, error) {
	if err := episode.start(failedGeneration, detected); err != nil {
		return nil, err
	}
	if episode.active != nil {
		return nil, errors.New("service Connection recovery proposal already retained")
	}
	if episode.proposals >= recoveryProposalLimit {
		return nil, errRecoveryExhausted
	}
	ticket := &recoveryProposal{generation: episode.generation + 1, deadline: episode.deadline}
	episode.proposals++
	episode.active = ticket
	return ticket, nil
}

// Loss starts the clock even before a replacement arrives. Waiting cannot
// postpone the original episode or reserve another proposal.
func (episode *recoveryEpisode) start(failedGeneration uint64, detected time.Time) error {
	if episode.stopped || detected.IsZero() || !detected.Before(episode.cutoff) {
		return errRecoveryExhausted
	}
	if failedGeneration != episode.generation || episode.generation == ^uint64(0) {
		return errIntegrity
	}
	if episode.deadline.IsZero() {
		start := episode.lastProgress
		if start.IsZero() || start.After(detected) {
			start = detected
		}
		episode.deadline = start.Add(recoveryEpisodeLimit).Add(-recoveryJoinReserve)
		if episode.cutoff.Before(episode.deadline) {
			episode.deadline = episode.cutoff
		}
	}
	if !detected.Before(episode.deadline) {
		return errRecoveryExhausted
	}
	return nil
}

// reject is called after this proposal's real physical work joins. Preserve
// that actual error; even a stopped owner must still retire its exact ticket.
func (episode *recoveryEpisode) reject(ticket *recoveryProposal, physicalErr error) error {
	if !episode.owns(ticket) || physicalErr == nil {
		return errors.Join(errIntegrity, physicalErr)
	}
	ticket.consumed = true
	episode.active = nil
	return physicalErr
}

// commit follows complete authentication, final live authority observations,
// and joining the old Attachment. Refusal retains the proposal for joined
// rejection; a late result cannot reset budgets or advance the generation.
func (episode *recoveryEpisode) commit(ticket *recoveryProposal, at time.Time) error {
	if !episode.owns(ticket) {
		return errIntegrity
	}
	if episode.stopped || at.IsZero() || !at.Before(ticket.deadline) || !at.Before(episode.cutoff) {
		return errRecoveryExhausted
	}
	ticket.consumed = true
	episode.active = nil
	episode.generation = ticket.generation
	episode.proposals = 0
	episode.deadline = time.Time{}
	return nil
}

// stop latches admission off while keeping any original proposal retained
// until its physical owner has joined and calls reject.
func (episode *recoveryEpisode) stop() { episode.stopped = true }

func (episode *recoveryEpisode) owns(ticket *recoveryProposal) bool {
	return ticket != nil && !ticket.consumed && episode.active == ticket &&
		episode.generation != ^uint64(0) && ticket.generation == episode.generation+1
}
