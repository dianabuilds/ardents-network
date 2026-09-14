package streamqualification

import "errors"

// Role is the fixed installed worker role. It is not an Application-selected
// identity and must agree with the root-installed systemd unit.
type Role byte

const (
	ReaderRole    Role = 1
	PublisherRole Role = 2
)

// Profile is a predeclared NET-14 workload shape. Profiles select only useful
// byte scheduling and canaries; they do not select a target, Route, authority
// or executable.
type Profile byte

const (
	ClientToPublisher Profile = 1
	PublisherToClient Profile = 2
)

// Schedule is the exact concurrent shape admitted by one profile.
type Schedule struct {
	OpenConnections   uint16
	ActiveConnections uint16
	AggregateBits     uint32
}

// Definition returns the fixed retained workload shape. A profile does not
// permit a caller to lower an individual Connection's required progress.
func (profile Profile) Definition(role Role) (Schedule, error) {
	if role != ReaderRole && role != PublisherRole {
		return Schedule{}, errors.New("qualification worker role is invalid")
	}
	switch profile {
	case ClientToPublisher, PublisherToClient:
		if role == ReaderRole {
			return Schedule{OpenConnections: 64, ActiveConnections: 16, AggregateBits: 10_000_000}, nil
		}
		return Schedule{OpenConnections: 256, ActiveConnections: 64, AggregateBits: 40_000_000}, nil
	default:
		return Schedule{}, errors.New("qualification workload profile is invalid")
	}
}
