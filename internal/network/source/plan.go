package source

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"time"
)

// Source is one indivisible Direct-Origin Source declaration. RootPEM is copied
// by New; Address, identity, family, endpoint handle, and leaf-key digest must
// all describe the same declared source.
type Source struct {
	Address        string
	ServerName     string
	Identity       [32]byte
	Family         string
	EndpointHandle string
	RootPEM        []byte
	LeafKeyDigest  [32]byte
}

// Config declares the complete finite Direct-Origin Source plan. Acquisition
// is either absent or exactly two fully declared sources. Serving is absent or
// a complete mutually authenticated endpoint. New copies PEM, certificates,
// and digest slices; any declared field requires its complete half, and a
// material index outside its bound is rejected. VerificationClock and
// MaterialIndex alone do not declare acquisition or serving. Addresses use a
// literal IP and nonzero numeric TCP port.
type Config struct {
	Sources           [2]Source
	ClientCertificate tls.Certificate
	MaterialIndex     uint32
	OrderSeed         [32]byte
	// VerificationClock is the State-owned verification time used only for
	// X.509 validation on configured Source TLS handshakes. A configured
	// client or listener without it is invalid.
	VerificationClock func() time.Time

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
	clock         func() time.Time
}

type server struct {
	address       string
	certificate   tls.Certificate
	clientRoots   *x509.CertPool
	clientDigests map[[32]byte]bool
	headerTimeout time.Duration
	clock         func() time.Time
}

// New validates and owns one complete source plan. Empty acquisition and
// serving halves are allowed; a partially configured half is rejected.
func New(input Config, authorities map[[32]byte]ed25519.PublicKey) (*Plan, Details, error) {
	plan := &Plan{details: Details{MaterialIndex: input.MaterialIndex, OrderSeed: input.OrderSeed}}
	if input.MaterialIndex >= 64 {
		return nil, Details{}, errors.New("source materialization index exceeds its bound")
	}
	if acquisitionDeclared(input) {
		if input.VerificationClock == nil {
			return nil, Details{}, errors.New("source verification clock is required")
		}
		if err := configureClients(plan, input, authorities); err != nil {
			return nil, Details{}, err
		}
	}
	if servingDeclared(input) {
		if input.VerificationClock == nil {
			return nil, Details{}, errors.New("source verification clock is required")
		}
		resolved, err := configureServer(input, authorities)
		if err != nil {
			return nil, Details{}, err
		}
		plan.server = resolved
		plan.details.Serving = true
	}
	return plan, plan.details, nil
}

func acquisitionDeclared(input Config) bool {
	if input.OrderSeed != [32]byte{} || certificateDeclared(input.ClientCertificate) {
		return true
	}
	for _, declared := range input.Sources {
		if declared.Address != "" || declared.ServerName != "" || declared.Identity != [32]byte{} ||
			declared.Family != "" || declared.EndpointHandle != "" || len(declared.RootPEM) != 0 ||
			declared.LeafKeyDigest != [32]byte{} {
			return true
		}
	}
	return false
}

func servingDeclared(input Config) bool {
	return input.ServeAddress != "" || certificateDeclared(input.ServeCertificate) ||
		len(input.ServeClientRootPEM) != 0 || len(input.ServeClientKeyDigests) != 0 ||
		input.ServeHeaderTimeout != 0
}

func certificateDeclared(certificate tls.Certificate) bool {
	return len(certificate.Certificate) != 0 || certificate.PrivateKey != nil || certificate.Leaf != nil ||
		len(certificate.OCSPStaple) != 0 || len(certificate.SignedCertificateTimestamps) != 0 ||
		len(certificate.SupportedSignatureAlgorithms) != 0
}

// Fetch performs one bounded authenticated request through a configured source.
func (p *Plan) Fetch(ctx context.Context, index int, request Message) (Message, error) {
	if p == nil || index < 0 || index >= len(p.clients) || p.clients[index].address == "" {
		return Message{}, errors.New("source index is not configured")
	}
	return fetch(ctx, p.clients[index], request)
}
