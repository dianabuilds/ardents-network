package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"time"
)

// ProbeConfig declares the private role-probe listener and its trust material.
// It is part of the Node's single duty lifecycle, not a separately callable
// transport module.
type ProbeConfig struct {
	ListenAddress string
	Certificate   tls.Certificate
	ClientRootPEM []byte
	ClientKeyPins [][32]byte
	MaximumDuty   time.Duration
	DrainTimeout  time.Duration
}

// probeDuty is the authenticated assignment served by one listener lifetime.
type probeDuty struct {
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

// probePlan is validated, owned role-probe configuration.
type probePlan struct {
	config ProbeConfig
	now    func() time.Time
}

// probeServer is the bounded capability handle for one running listener.
type probeServer struct {
	Done    <-chan error
	Protect func(bool)
	Usage   func() (uint64, uint64, uint64)
	Stop    func()
	Drain   func(context.Context) error
}

// newProbePlan validates and owns the Node's private role-probe listener plan.
func newProbePlan(input ProbeConfig, identity ed25519.PublicKey, now func() time.Time) (*probePlan, error) {
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
	if err := cloneProbeTLSMaterial(&input, identity, now().UTC()); err != nil {
		return nil, err
	}
	return &probePlan{config: input, now: func() time.Time { return now().UTC() }}, nil
}
