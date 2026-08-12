package node

import "fmt"

type lifecycleState byte

const (
	stateAbsent lifecycleState = iota
	statePrepared
	stateReady
	stateDraining
	stateWithdrawn
	stateFailed
)

var stateNames = [...]string{"ABSENT", "PREPARED", "READY", "DRAINING", "WITHDRAWN", "FAILED"}

type stateMachine struct{ current lifecycleState }

func (m *stateMachine) move(next lifecycleState) error {
	if m.current == next {
		return nil
	}
	allowed := false
	switch m.current {
	case stateAbsent:
		allowed = next == statePrepared || next == stateFailed
	case statePrepared:
		allowed = next == stateReady || next == stateFailed
	case stateReady:
		allowed = next == stateDraining || next == stateFailed
	case stateDraining:
		allowed = next == stateWithdrawn || next == stateFailed
	}
	if !allowed {
		return fmt.Errorf("illegal Node lifecycle transition %s -> %s", m.name(), stateNames[next])
	}
	m.current = next
	return nil
}

func (m stateMachine) name() string { return stateNames[m.current] }
