package diagnostics

import (
	"fmt"
	"time"
)

type LifecycleTransitionSnapshot struct {
	From string
	To   string
	At   time.Time
}

type LifecycleSnapshot struct {
	Current        string
	Previous       string
	EnteredAt      time.Time
	TransitionedAt time.Time
	Transitions    []LifecycleTransitionSnapshot
}

const (
	Stopped      = "stopped"
	Starting     = "starting"
	Initializing = "initializing"
	Ready        = "ready"
	Degraded     = "degraded"
	Stopping     = "stopping"
	Failed       = "failed"
)

type Machine struct {
	state          string
	previous       string
	enteredAt      time.Time
	transitionedAt time.Time
	transitions    []LifecycleTransitionSnapshot
}

func NewMachine() *Machine       { return &Machine{state: Stopped, enteredAt: time.Now().UTC()} }
func (m *Machine) State() string { return m.state }

func (m *Machine) Move(next string) error {
	if m.state == next {
		return nil
	}
	if !allowedLifecycleTransition(m.state, next) {
		return fmt.Errorf("invalid lifecycle transition %s -> %s", m.state, next)
	}
	now := time.Now().UTC()
	m.transitions = append(m.transitions, LifecycleTransitionSnapshot{From: m.state, To: next, At: now})
	m.previous, m.state, m.enteredAt, m.transitionedAt = m.state, next, now, now
	return nil
}

func (m *Machine) Snapshot() LifecycleSnapshot {
	return LifecycleSnapshot{Current: m.state, Previous: m.previous, EnteredAt: m.enteredAt,
		TransitionedAt: m.transitionedAt, Transitions: append([]LifecycleTransitionSnapshot(nil), m.transitions...)}
}

func APISnapshot(snapshot LifecycleSnapshot) LifecycleSnapshot {
	snapshot.Transitions = append([]LifecycleTransitionSnapshot(nil), snapshot.Transitions...)
	return snapshot
}

func allowedLifecycleTransition(current, next string) bool {
	switch current {
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
