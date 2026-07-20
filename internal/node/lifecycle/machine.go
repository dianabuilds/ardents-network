package lifecycle

import (
	"fmt"
	"time"
)

const (
	Stopped      = "stopped"
	Starting     = "starting"
	Initializing = "initializing"
	Ready        = "ready"
	Degraded     = "degraded"
	Stopping     = "stopping"
	Failed       = "failed"
)

type Transition struct {
	From string    `json:"from"`
	To   string    `json:"to"`
	At   time.Time `json:"at"`
}

type Snapshot struct {
	Current        string       `json:"current"`
	Previous       string       `json:"previous,omitempty"`
	EnteredAt      time.Time    `json:"entered_at"`
	TransitionedAt time.Time    `json:"transitioned_at,omitempty"`
	Transitions    []Transition `json:"transitions,omitempty"`
}

type Machine struct {
	state          string
	previous       string
	enteredAt      time.Time
	transitionedAt time.Time
	transitions    []Transition
}

func NewMachine() *Machine {
	now := time.Now().UTC()
	return &Machine{state: Stopped, enteredAt: now}
}

func (m *Machine) State() string {
	return m.state
}

func (m *Machine) Move(next string) error {
	if m.state == next {
		return nil
	}
	if !allowed(m.state, next) {
		return fmt.Errorf("invalid lifecycle transition %s -> %s", m.state, next)
	}
	now := time.Now().UTC()
	m.transitions = append(m.transitions, Transition{From: m.state, To: next, At: now})
	m.previous = m.state
	m.state = next
	m.enteredAt = now
	m.transitionedAt = now
	return nil
}

func (m *Machine) Snapshot() Snapshot {
	out := Snapshot{
		Current:        m.state,
		Previous:       m.previous,
		EnteredAt:      m.enteredAt,
		TransitionedAt: m.transitionedAt,
	}
	if len(m.transitions) > 0 {
		out.Transitions = make([]Transition, len(m.transitions))
		copy(out.Transitions, m.transitions)
	}
	return out
}

func allowed(cur, next string) bool {
	switch cur {
	case Stopped:
		return next == Starting
	case Starting:
		return next == Initializing || next == Failed
	case Initializing:
		return next == Ready || next == Degraded || next == Failed
	case Ready:
		return next == Degraded || next == Stopping || next == Failed
	case Degraded:
		return next == Ready || next == Stopping || next == Failed
	case Stopping:
		return next == Stopped || next == Failed
	case Failed:
		return next == Starting
	default:
		return false
	}
}
