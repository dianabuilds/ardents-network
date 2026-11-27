package transfer

import (
	"errors"
	"time"
)

const (
	recoveryProposalLimit = 3
	recoveryEpisodeLimit  = 15 * time.Second
	recoveryJoinReserve   = 10 * time.Millisecond
)

// ErrRecoveryExhausted identifies the exhausted original time or proposal bound.
var ErrRecoveryExhausted = errors.New("service Connection recovery bound reached")

// RecoveryEpisode retains the finite proposal budget under the stream lock.
// It is a scheduling mechanism, not an opener or authorization. The stream
// must check the original operation, Network and signed bounds around every
// effect, authenticate Continuity, and join rejected physical work. A ticket
// neither proves that work joined nor grants a replacement Attachment.
type RecoveryEpisode struct {
	generation   uint64
	lastProgress time.Time
	cutoff       time.Time
	deadline     time.Time
	proposals    int
	active       *RecoveryTicket
	stopped      bool
}

// RecoveryTicket identifies one retained proposal for the next generation.
type RecoveryTicket struct {
	generation uint64
	deadline   time.Time
	retry      bool
	consumed   bool
}

// NewRecoveryEpisode starts with the already admitted cutoff for original no-new-recovery
// authority. It may not be renewed when an Attachment or episode changes.
func NewRecoveryEpisode(generation uint64, started, cutoff time.Time) (*RecoveryEpisode, error) {
	if generation == 0 || started.IsZero() || !started.Before(cutoff) {
		return nil, ErrIntegrity
	}
	return &RecoveryEpisode{generation: generation, lastProgress: started, cutoff: cutoff}, nil
}

// Progress records actual authenticated byte/control progress, never an open
// attempt or physical retry. Progress cannot extend an already started episode.
func (episode *RecoveryEpisode) Progress(at time.Time) {
	if !episode.stopped && at.After(episode.lastProgress) {
		episode.lastProgress = at
	}
}

// Begin spends a proposal and retains its ticket until Commit or joined Reject.
func (episode *RecoveryEpisode) Begin(failedGeneration uint64, detected time.Time) (*RecoveryTicket, error) {
	if err := episode.Start(failedGeneration, detected); err != nil {
		return nil, err
	}
	if episode.active != nil {
		return nil, errors.New("service Connection recovery proposal already retained")
	}
	if episode.proposals >= recoveryProposalLimit {
		return nil, ErrRecoveryExhausted
	}
	ticket := &RecoveryTicket{generation: episode.generation + 1, deadline: episode.deadline, retry: episode.proposals > 0}
	episode.proposals++
	episode.active = ticket
	return ticket, nil
}

// Start records loss and starts the clock even before a replacement arrives. Waiting cannot
// postpone the original episode or reserve another proposal.
func (episode *RecoveryEpisode) Start(failedGeneration uint64, detected time.Time) error {
	if episode.stopped || detected.IsZero() || !detected.Before(episode.cutoff) {
		return ErrRecoveryExhausted
	}
	if failedGeneration != episode.generation || episode.generation == ^uint64(0) {
		return ErrIntegrity
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
		return ErrRecoveryExhausted
	}
	return nil
}

// Reject is called after this proposal's real physical work joins. Preserve
// that actual error; even a stopped owner must still retire its exact ticket.
func (episode *RecoveryEpisode) Reject(ticket *RecoveryTicket, physicalErr error) error {
	if !episode.Owns(ticket) || physicalErr == nil {
		return errors.Join(ErrIntegrity, physicalErr)
	}
	ticket.consumed = true
	episode.active = nil
	return physicalErr
}

// Commit follows complete authentication, final live authority observations,
// and joining the old Attachment. Refusal retains the proposal for joined
// rejection; a late result cannot reset budgets or advance the generation.
func (episode *RecoveryEpisode) Commit(ticket *RecoveryTicket, at time.Time) error {
	if !episode.Owns(ticket) {
		return ErrIntegrity
	}
	if episode.stopped || at.IsZero() || !at.Before(ticket.deadline) || !at.Before(episode.cutoff) {
		return ErrRecoveryExhausted
	}
	ticket.consumed = true
	episode.active = nil
	episode.generation = ticket.generation
	episode.proposals = 0
	episode.deadline = time.Time{}
	return nil
}

// Stop latches admission off while keeping any original proposal retained
// until its physical owner has joined and calls reject.
func (episode *RecoveryEpisode) Stop() { episode.stopped = true }

// Owns checks the original unconsumed ticket at the current generation.
func (episode *RecoveryEpisode) Owns(ticket *RecoveryTicket) bool {
	return ticket != nil && !ticket.consumed && episode.active == ticket &&
		episode.generation != ^uint64(0) && ticket.generation == episode.generation+1
}

// Deadline returns the original deadline fixed by Start.
func (episode *RecoveryEpisode) Deadline() time.Time { return episode.deadline }

// CanRetry reports whether the bound permits another proposal after retirement.
func (episode *RecoveryEpisode) CanRetry(at time.Time) bool {
	return !episode.stopped && episode.proposals < recoveryProposalLimit && at.Before(episode.deadline)
}

// Generation returns the Attachment generation reserved by the proposal.
func (ticket *RecoveryTicket) Generation() uint64 { return ticket.generation }

// Deadline returns the episode deadline copied into the ticket at admission.
func (ticket *RecoveryTicket) Deadline() time.Time { return ticket.deadline }

// IsRetry reports whether this proposal followed another in the same episode.
func (ticket *RecoveryTicket) IsRetry() bool { return ticket.retry }
