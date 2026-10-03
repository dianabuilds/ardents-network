package admission

import "time"

// Class identifies the finite kind of work paid for by an admission token.
// Its numeric value is part of the existing closed token and wire contracts.
type Class uint8

const (
	ControlClass      Class = 1
	ForwardClass      Class = 2
	RegistrationClass Class = 3
)

// ByteLimit is the complete bidirectional allowance of one admission. A
// registration includes its exchange, retained deliveries and withdrawal.
// An unsupported class grants no allowance.
func (class Class) ByteLimit() uint64 {
	switch class {
	case ControlClass:
		return 64 << 10
	case ForwardClass:
		return 32 << 20
	case RegistrationClass:
		return 8 << 20
	}
	return 0
}

// Lifetime bounds an admission independently of shorter caller and authority
// deadlines. An unsupported class grants no lifetime.
func (class Class) Lifetime() time.Duration {
	switch class {
	case ControlClass:
		return 30 * time.Second
	case ForwardClass:
		return 1800 * time.Second
	case RegistrationClass:
		return 600 * time.Second
	}
	return 0
}
