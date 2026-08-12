package probe

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"time"
)

// Config declares one private role-probe listener and its trust material.
type Config struct {
	ListenAddress string
	Certificate   tls.Certificate
	ClientRootPEM []byte
	ClientKeyPins [][32]byte
	MaximumDuty   time.Duration
	DrainTimeout  time.Duration
}

// Duty is the authenticated assignment served by one listener lifetime.
type Duty struct {
	NetworkID        [32]byte
	EpochDigest      [32]byte
	NodeID           [32]byte
	AssignmentDigest [32]byte
	EpochValidFrom   time.Time
	EpochValidUntil  time.Time
	RecordValidFrom  time.Time
	RecordValidUntil time.Time
	Capacity         uint16
}

// Plan is validated, owned role-probe configuration.
type Plan struct {
	config Config
	now    func() time.Time
}

// Server is the bounded capability handle for one running listener.
type Server struct {
	Done    <-chan error
	Protect func(bool)
	Usage   func() (uint64, uint64, uint64)
	Stop    func()
	Drain   func(context.Context)
}

// New validates and owns the role-probe listener plan.
func New(input Config, identity ed25519.PublicKey, now func() time.Time) (*Plan, error) {
	if len(identity) != ed25519.PublicKeySize || input.ListenAddress == "" {
		return nil, errors.New("role-probe identity and listener are required")
	}
	host, port, err := net.SplitHostPort(input.ListenAddress)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || net.ParseIP(host) == nil || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return nil, errors.New("role-probe endpoint must be a literal IP and port")
	}
	if len(input.ClientRootPEM) == 0 || len(input.ClientRootPEM) > 64<<10 ||
		len(input.ClientKeyPins) == 0 || len(input.ClientKeyPins) > 16 {
		return nil, errors.New("role-probe client trust is invalid")
	}
	seenPins := make(map[[32]byte]bool, len(input.ClientKeyPins))
	for _, pin := range input.ClientKeyPins {
		if pin == [32]byte{} || seenPins[pin] {
			return nil, errors.New("role-probe client key pins are invalid")
		}
		seenPins[pin] = true
	}
	if input.MaximumDuty <= 0 || input.MaximumDuty > 15*time.Second ||
		input.DrainTimeout <= 0 || input.DrainTimeout > 15*time.Second {
		return nil, errors.New("role-probe time bounds are invalid")
	}
	if now == nil {
		now = time.Now
	}
	if err := cloneTLSMaterial(&input, identity, now().UTC()); err != nil {
		return nil, err
	}
	return &Plan{config: input, now: func() time.Time { return now().UTC() }}, nil
}

// ListenAddress returns the literal endpoint owned by the plan.
func (p *Plan) ListenAddress() string { return p.config.ListenAddress }

// MaximumDuty returns the longest accepted connection lifetime.
func (p *Plan) MaximumDuty() time.Duration { return p.config.MaximumDuty }
