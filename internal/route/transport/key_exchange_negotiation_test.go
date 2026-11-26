package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"net"
	"slices"
	"testing"
	"time"
)

// These controls observe the actual crypto/tls negotiation of the production
// Route TLS constructors against an independent peer. The peer pins its own
// literal group list and never reads the production list, so a production
// regression cannot tune its own oracle. They establish no Network
// observation, Admission right or Route operation.

var negotiationGroups = []tls.CurveID{tls.X25519MLKEM768, tls.X25519}

func keyExchangeCertificate(t *testing.T, now time.Time) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(private) })
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	var key [32]byte
	copy(key[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}, key
}

// keyExchangePeerServer serves one exact version and group list. A requested
// client certificate is only demanded, never authenticated: production owns that.
func keyExchangePeerServer(certificate tls.Certificate, requestClientCertificate bool, groups []tls.CurveID, version uint16, protocol string) *tls.Config {
	config := &tls.Config{MinVersion: version, MaxVersion: version, NextProtos: []string{protocol}, CurvePreferences: groups, Certificates: []tls.Certificate{certificate}}
	if requestClientCertificate {
		config.ClientAuth = tls.RequireAnyClientCert
	}
	return config
}

// keyExchangePeerClient offers one exact version and group list. It verifies
// no peer itself; the production side under test owns every identity check.
func keyExchangePeerClient(certificate *tls.Certificate, groups []tls.CurveID, version uint16, protocol string, resume bool) *tls.Config {
	config := &tls.Config{MinVersion: version, MaxVersion: version, InsecureSkipVerify: true, NextProtos: []string{protocol}, CurvePreferences: groups}
	if certificate != nil {
		config.Certificates = []tls.Certificate{*certificate}
	}
	if resume {
		config.ClientSessionCache = tls.NewLRUClientSessionCache(1)
	}
	return config
}

type keyExchangeNegotiation struct {
	production, peer   tls.ConnectionState
	productionErr, err error
}

// negotiateKeyExchange joins both handshake owners before returning, whether
// the production side accepted or refused, and retains each side's result.
func negotiateKeyExchange(t *testing.T, production, peer *tls.Config, productionIsClient bool) keyExchangeNegotiation {
	t.Helper()
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	end := time.Now().Add(10 * time.Second)
	if err := errors.Join(local.SetDeadline(end), remote.SetDeadline(end)); err != nil {
		t.Fatal(err)
	}
	productionConnection, peerConnection := tls.Server(local, production), tls.Client(remote, peer)
	if productionIsClient {
		productionConnection, peerConnection = tls.Client(local, production), tls.Server(remote, peer)
	}
	joined := make(chan error, 1)
	go func() { joined <- peerConnection.HandshakeContext(context.Background()) }()
	result := keyExchangeNegotiation{productionErr: productionConnection.HandshakeContext(t.Context())}
	// Release a peer still waiting for a flight a refusal never sends.
	local.Close()
	remote.Close()
	result.err = <-joined
	result.production, result.peer = productionConnection.ConnectionState(), peerConnection.ConnectionState()
	return result
}

type keyExchangeSide struct {
	name         string
	client       bool // Production dials; otherwise it serves.
	node, shared bool // A Node certificate is used; the shared listener classifies it.
	config       func() *tls.Config
}

func keyExchangeSides(serverCertificate tls.Certificate, serverKey [32]byte, nodeCertificate tls.Certificate) []keyExchangeSide {
	return []keyExchangeSide{
		{"role client", true, false, false, func() *tls.Config { return RoleClientTLS(serverKey) }},
		{"node client", true, true, false, func() *tls.Config { return NodeClientTLS(nodeCertificate, serverKey) }},
		{"role server", false, false, false, func() *tls.Config { return RoleServerTLS(serverCertificate) }},
		{"shared server direct", false, false, true, func() *tls.Config { return SharedServerTLS(serverCertificate) }},
		{"shared server node", false, true, true, func() *tls.Config { return SharedServerTLS(serverCertificate) }},
	}
}

// keyExchangeOpposite is the independent peer of one production side.
func keyExchangeOpposite(side keyExchangeSide, serverCertificate, nodeCertificate tls.Certificate, groups []tls.CurveID, version uint16, protocol string) *tls.Config {
	if side.client {
		return keyExchangePeerServer(serverCertificate, side.node, groups, version, protocol)
	}
	if side.node {
		return keyExchangePeerClient(&nodeCertificate, groups, version, protocol, false)
	}
	return keyExchangePeerClient(nil, groups, version, protocol, false)
}

func TestClosedProfileKeyExchangeNegotiationMatrix(t *testing.T) {
	now := time.Now()
	serverCertificate, serverKey := keyExchangeCertificate(t, now)
	nodeCertificate, nodeKey := keyExchangeCertificate(t, now)
	peers := []struct {
		name, protocol string
		groups         []tls.CurveID
		version        uint16
		want           tls.CurveID // Zero refuses before any Route frame.
	}{
		{"both groups", ClosedRouteProfile, negotiationGroups, tls.VersionTLS13, tls.X25519MLKEM768},
		{"hybrid group only", ClosedRouteProfile, []tls.CurveID{tls.X25519MLKEM768}, tls.VersionTLS13, tls.X25519MLKEM768},
		{"classical group only", ClosedRouteProfile, []tls.CurveID{tls.X25519}, tls.VersionTLS13, tls.X25519},
		{"peer order reversed", ClosedRouteProfile, []tls.CurveID{tls.X25519, tls.X25519MLKEM768}, tls.VersionTLS13, tls.X25519MLKEM768},
		{"P-256 only", ClosedRouteProfile, []tls.CurveID{tls.CurveP256}, tls.VersionTLS13, 0},
		{"P-384 only", ClosedRouteProfile, []tls.CurveID{tls.CurveP384}, tls.VersionTLS13, 0},
		{"TLS 1.2 only", ClosedRouteProfile, negotiationGroups, tls.VersionTLS12, 0},
		{"foreign ALPN", "ardents-route-v2", negotiationGroups, tls.VersionTLS13, 0},
	}
	for _, side := range keyExchangeSides(serverCertificate, serverKey, nodeCertificate) {
		for _, peer := range peers {
			t.Run(side.name+"/"+peer.name, func(t *testing.T) {
				result := negotiateKeyExchange(t, side.config(), keyExchangeOpposite(side, serverCertificate, nodeCertificate, peer.groups, peer.version, peer.protocol), side.client)
				if peer.want == 0 {
					if result.productionErr == nil || result.production.HandshakeComplete {
						t.Fatalf("unoffered negotiation completed: curve=%d version=%#x", result.production.CurveID, result.production.Version)
					}
					t.Logf("refused before completion: %v", result.productionErr)
					return
				}
				if result.productionErr != nil || result.err != nil {
					// A hybrid miss on "both groups" means GODEBUG tlsmlkem=0 in
					// this environment: invalid here, never a passing skip.
					t.Fatalf("allowed negotiation refused: production=%v peer=%v", result.productionErr, result.err)
				}
				for name, state := range map[string]tls.ConnectionState{"production": result.production, "peer": result.peer} {
					if state.CurveID != peer.want || state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile || state.DidResume {
						t.Fatalf("%s negotiated curve=%d version=%#x ALPN=%q resumed=%t", name, state.CurveID, state.Version, state.NegotiatedProtocol, state.DidResume)
					}
				}
				certificates := 0
				if side.client || side.node {
					certificates = 1
				}
				if len(result.production.PeerCertificates) != certificates {
					t.Fatalf("production peer certificates = %d, want %d", len(result.production.PeerCertificates), certificates)
				}
				t.Logf("negotiated curve=%d version=%#x ALPN=%q peer-certificates=%d", result.production.CurveID, result.production.Version, result.production.NegotiatedProtocol, len(result.production.PeerCertificates))
				if !side.shared {
					return
				}
				carrier, err := ClassifySharedTLS(result.production, func(key [32]byte) bool { return key == nodeKey })
				if !side.node {
					if err != nil || carrier.Kind != ClosedSharedDirect {
						t.Fatalf("certificate-free role classified %#v: %v", carrier, err)
					}
					return
				}
				if err != nil || carrier.Kind != ClosedSharedNode || carrier.NodeKey != nodeKey {
					t.Fatalf("mutual Node classified %#v: %v", carrier, err)
				}
				if other, err := ClassifySharedTLS(result.production, func([32]byte) bool { return false }); err == nil || other.Kind == ClosedSharedDirect {
					t.Fatalf("unauthorized Node became direct: %#v %v", other, err)
				}
			})
		}
	}
}

func TestClosedProfileKeyExchangeClientOfferIsExactPair(t *testing.T) {
	now := time.Now()
	serverCertificate, serverKey := keyExchangeCertificate(t, now)
	nodeCertificate, _ := keyExchangeCertificate(t, now)
	for _, side := range keyExchangeSides(serverCertificate, serverKey, nodeCertificate)[:2] {
		t.Run(side.name, func(t *testing.T) {
			var offered struct {
				groups    []tls.CurveID
				versions  []uint16
				protocols []string
			}
			calls := 0
			peer := keyExchangeOpposite(side, serverCertificate, nodeCertificate, negotiationGroups, tls.VersionTLS13, ClosedRouteProfile)
			peer.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
				calls++
				offered.groups, offered.versions, offered.protocols = slices.Clone(hello.SupportedCurves), slices.Clone(hello.SupportedVersions), slices.Clone(hello.SupportedProtos)
				return nil, nil
			}
			result := negotiateKeyExchange(t, side.config(), peer, true)
			if result.productionErr != nil || result.err != nil || calls != 1 {
				t.Fatalf("client hello was not observed once: production=%v peer=%v calls=%d", result.productionErr, result.err, calls)
			}
			if !slices.Equal(offered.groups, negotiationGroups) || !slices.Equal(offered.versions, []uint16{tls.VersionTLS13}) || !slices.Equal(offered.protocols, []string{ClosedRouteProfile}) {
				t.Fatalf("client offered groups=%v versions=%v protocols=%v", offered.groups, offered.versions, offered.protocols)
			}
		})
	}
}

func TestClosedProfileKeyExchangeNeverResumes(t *testing.T) {
	now := time.Now()
	serverCertificate, serverKey := keyExchangeCertificate(t, now)
	nodeCertificate, _ := keyExchangeCertificate(t, now)
	for _, side := range keyExchangeSides(serverCertificate, serverKey, nodeCertificate) {
		t.Run(side.name, func(t *testing.T) {
			var peer *tls.Config
			if side.client {
				// The peer keeps offering resumption; production never stores a session.
				peer = keyExchangePeerServer(serverCertificate, side.node, negotiationGroups, tls.VersionTLS13, ClosedRouteProfile)
			} else {
				// The peer would resume; production issues no ticket.
				peer = keyExchangePeerClient(nil, negotiationGroups, tls.VersionTLS13, ClosedRouteProfile, true)
				if side.node {
					peer.Certificates = []tls.Certificate{nodeCertificate}
				}
			}
			for round := 1; round <= 2; round++ {
				result := negotiateKeyExchange(t, side.config(), peer, side.client)
				if result.productionErr != nil || result.err != nil {
					t.Fatalf("round %d refused: production=%v peer=%v", round, result.productionErr, result.err)
				}
				if result.production.DidResume || result.peer.DidResume {
					t.Fatalf("round %d resumed: production=%t peer=%t", round, result.production.DidResume, result.peer.DidResume)
				}
			}
		})
	}
}

// Every constructor pins the independent pair and owns its slice: changing one
// configuration never changes another held or later configuration.
func TestClosedProfileKeyExchangeConfigsPinIndependentPair(t *testing.T) {
	now := time.Now()
	serverCertificate, serverKey := keyExchangeCertificate(t, now)
	nodeCertificate, _ := keyExchangeCertificate(t, now)
	constructors := []func() *tls.Config{
		func() *tls.Config { return RoleClientTLS(serverKey) },
		func() *tls.Config { return RoleServerTLS(serverCertificate) },
		func() *tls.Config { return NodeClientTLS(nodeCertificate, serverKey) },
		func() *tls.Config { return SharedServerTLS(serverCertificate) },
	}
	held := make([]*tls.Config, len(constructors))
	for index, construct := range constructors {
		held[index] = construct()
	}
	for index, construct := range constructors {
		config := construct()
		if !slices.Equal(config.CurvePreferences, negotiationGroups) {
			t.Fatalf("constructor %d configured %v, want the closed profile pair", index, config.CurvePreferences)
		}
		config.CurvePreferences[0], config.CurvePreferences[1] = tls.CurveP256, tls.CurveP384
		for other, otherConfig := range held {
			if !slices.Equal(otherConfig.CurvePreferences, negotiationGroups) {
				t.Fatalf("constructor %d changed held constructor %d", index, other)
			}
		}
		if later := construct(); !slices.Equal(later.CurvePreferences, negotiationGroups) {
			t.Fatalf("constructor %d changed a later configuration", index)
		}
	}
}
