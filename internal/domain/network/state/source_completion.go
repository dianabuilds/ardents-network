package state

import (
	"errors"
	"sync"
)

// SourceCompletion retains this original serving producer beyond State.Close
// while command-owned inputs, observations and process protection join. Copies
// share the same one-shot outcome. It grants no software or recovery authority.
type SourceCompletion struct{ original *sourceCompletion }

type sourceCompletion struct {
	owner  *State
	joined chan error
	once   sync.Once
	result error
}

// RetainSourceCompletion is available once on the original serving State before
// physical Close starts. An already latched retirement failure is not cleared.
// Once retained, Close joins State and releases its storage lease but preserves
// the producer. A new Source cannot claim it; inspection also refuses it.
func (s *State) RetainSourceCompletion() (SourceCompletion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing || !s.config.sourceInfo.Serving || s.serverDone == nil || s.sourceCompletion != nil {
		return SourceCompletion{}, errors.New("source completion retention unavailable")
	}
	c := &sourceCompletion{owner: s, joined: make(chan error, 1)}
	s.sourceCompletion = c
	return SourceCompletion{original: c}, nil
}

// Finish consumes the original State join and the command's actual additional
// terminal outcome. A supplied nil cannot override a failed/unjoined State.
// State/command failure preserves the live producer, including after expiry.
// Final native removal/close errors remain errors; the participation owner
// retains its durable commit/uncertainty rules. This supplies no reset/adoption.
// Call only after State.Close
// and every command-owned observation/input/process user have joined.
func (c SourceCompletion) Finish(commandResult error) error {
	if c.original == nil {
		return errors.New("source completion absent")
	}
	o := c.original
	o.once.Do(func() {
		select {
		case stateResult := <-o.joined:
			o.result = errors.Join(stateResult, commandResult)
			if o.result == nil {
				o.result = o.owner.releaseSourceServer()
			}
		default:
			o.result = errors.Join(errors.New("original Source State has not joined"), commandResult)
		}
	})
	return o.result
}
