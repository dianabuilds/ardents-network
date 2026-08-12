package source

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"time"
)

// Config declares the complete finite Direct-Origin Source plan.
type Config struct {
	Addresses         [2]string
	ServerNames       [2]string
	Identities        [2][32]byte
	Families          [2]string
	EndpointHandles   [2]string
	RootPEM           [2][]byte
	LeafKeyDigests    [2][32]byte
	ClientCertificate tls.Certificate
	MaterialIndex     uint32
	OrderSeed         [32]byte

	ServeAddress          string
	ServeCertificate      tls.Certificate
	ServeClientRootPEM    []byte
	ServeClientKeyDigests [][32]byte
	ServeHeaderTimeout    time.Duration
}

// Details are the immutable non-secret facts of a validated source plan.
type Details struct {
	Configured      bool
	Serving         bool
	MaterialIndex   uint32
	OrderSeed       [32]byte
	Identities      [2][32]byte
	Families        [2]string
	EndpointHandles [2]string
	Exposures       [2][32]byte
}

// Plan is a validated, owned source acquisition and distribution plan.
type Plan struct {
	clients [2]client
	details Details
	server  server
}

type client struct {
	address       string
	serverName    string
	roots         *x509.CertPool
	leafKeyDigest [32]byte
	certificate   tls.Certificate
}

type server struct {
	address       string
	certificate   tls.Certificate
	clientRoots   *x509.CertPool
	clientDigests map[[32]byte]bool
	headerTimeout time.Duration
}

// New validates and owns one complete source plan. Empty acquisition and
// serving halves are allowed; a partially configured half is rejected.
func New(input Config, authorities map[[32]byte]ed25519.PublicKey) (*Plan, error) {
	plan := &Plan{details: Details{MaterialIndex: input.MaterialIndex, OrderSeed: input.OrderSeed}}
	if input.MaterialIndex >= 64 {
		return nil, errors.New("source materialization index exceeds its bound")
	}
	if input.Addresses[0] != "" || input.Addresses[1] != "" {
		if err := configureClients(plan, input, authorities); err != nil {
			return nil, err
		}
	}
	if input.ServeAddress != "" {
		resolved, err := configureServer(input, authorities)
		if err != nil {
			return nil, err
		}
		plan.server = resolved
		plan.details.Serving = true
	}
	return plan, nil
}

// Details returns the immutable non-secret plan facts.
func (p *Plan) Details() Details { return p.details }

// Fetch performs one bounded authenticated request through a configured source.
func (p *Plan) Fetch(ctx context.Context, index int, request Message) (Message, error) {
	if p == nil || index < 0 || index >= len(p.clients) || p.clients[index].address == "" {
		return Message{}, errors.New("source index is not configured")
	}
	return fetch(ctx, p.clients[index], request)
}
