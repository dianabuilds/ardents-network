//go:build referencec2

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	alphaprivate "github.com/dianabuilds/ardents-network/internal/naming/alpha/private"
)

const (
	alphaGatewayReadySchema = "ardents-e2e-alpha-gateway-ready-v1"
	alphaRelayReadySchema   = "ardents-e2e-alpha-relay-ready-v1"
)

// alphaGatewayReady is a test-only handoff from the alpha Gateway process to
// the Relay and User processes. It exposes only the Gateway's public OHTTP
// profile and fixture TLS root, never a corpus-signing key or Target.
type alphaGatewayReady struct {
	Schema, URL, ServerName, RootPEM, GatewayPublic string
	Profile                                         alphaprivate.GatewayProfile
}

// alphaRelayReady is a test-only handoff from the Relay process to a User.
// The client learns no Gateway location from it.
type alphaRelayReady struct {
	Schema, URL, ServerName, RootPEM, GatewayPublic string
	Profile                                         alphaprivate.GatewayProfile
}

func runAlphaGateway(input config) error {
	deadline, _ := input.deadline()
	envelope, err := waitForPublication(context.Background(), input.PublicationPath, deadline)
	if err != nil {
		return err
	}
	authority, err := input.alphaCorpusAuthority()
	if err != nil {
		return err
	}
	raw, err := base64.RawStdEncoding.DecodeString(envelope.AlphaCorpus)
	if err != nil {
		return errors.New("C2 alpha Gateway corpus is invalid")
	}
	corpus, err := alpha.OpenCorpus(authority, raw)
	if err != nil || corpus.Cohort() != "reference-c2" {
		return errors.New("C2 alpha Gateway corpus is unavailable")
	}
	identityPublic, identityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	gateway, err := alphaprivate.NewGateway(alphaprivate.GatewayConfig{Corpus: corpus, NodeID: sha256.Sum256(identityPublic),
		Family: "reference-c2-alpha-gateway", AssignmentNotAfter: deadline, IdentityKey: identityPrivate, Clock: time.Now})
	if err != nil {
		return err
	}
	server, serve, address, serverName, root, err := openFixtureAlphaServer(gateway.Handler(), "alpha-gateway.fixture.invalid", "")
	if err != nil {
		return err
	}
	ready := alphaGatewayReady{Schema: alphaGatewayReadySchema, URL: "https://" + address, ServerName: serverName,
		RootPEM: base64.RawStdEncoding.EncodeToString(root), GatewayPublic: hex.EncodeToString(identityPublic), Profile: gateway.Profile()}
	if err := writeAlphaReady(input.AlphaGatewayReadyPath, ready); err != nil {
		_ = server.Close()
		return err
	}
	return closeFixtureAlphaServer(input, server, serve, "alpha-gateway")
}

func runAlphaRelay(input config) error {
	deadline, _ := input.deadline()
	gateway, err := waitAlphaGatewayReady(input.AlphaGatewayReadyPath, deadline)
	if err != nil {
		return err
	}
	transport, err := alphaClientTransport(gateway.RootPEM, gateway.ServerName)
	if err != nil {
		return err
	}
	relay, err := alphaprivate.NewRelay(gateway.URL, &http.Client{Transport: transport})
	if err != nil {
		return err
	}
	server, serve, address, serverName, root, err := openFixtureAlphaServer(relay.Handler(), "alpha-relay.fixture.invalid", input.AlphaRelayListenAddress)
	if err != nil {
		return err
	}
	ready := alphaRelayReady{Schema: alphaRelayReadySchema, URL: "https://" + address, ServerName: serverName,
		RootPEM: base64.RawStdEncoding.EncodeToString(root), GatewayPublic: gateway.GatewayPublic, Profile: gateway.Profile}
	if err := writeAlphaReady(input.AlphaRelayReadyPath, ready); err != nil {
		_ = server.Close()
		return err
	}
	return closeFixtureAlphaServer(input, server, serve, "alpha-relay")
}

func (input config) openPrivateAlpha(floor *alpha.PersistentFloor, network [32]byte, at time.Time) (*alphaprivate.Client, error) {
	if floor == nil || at.IsZero() {
		return nil, errors.New("C2 alpha Endpoint private resolution input is invalid")
	}
	deadline, _ := input.deadline()
	relay, err := waitAlphaRelayReady(input.AlphaRelayReadyPath, deadline)
	if err != nil {
		return nil, err
	}
	authority, err := input.alphaCorpusAuthority()
	if err != nil {
		return nil, err
	}
	gatewayPublic, err := hex.DecodeString(relay.GatewayPublic)
	if err != nil || len(gatewayPublic) != ed25519.PublicKeySize {
		return nil, errors.New("C2 alpha Gateway identity is invalid")
	}
	transport, err := alphaClientTransport(relay.RootPEM, relay.ServerName)
	if err != nil {
		return nil, err
	}
	client, err := alphaprivate.Open(alphaprivate.ClientConfig{RelayURL: relay.URL, RelayNodeID: [32]byte{1}, RelayFamily: "reference-c2-alpha-relay",
		GatewayPublic: ed25519.PublicKey(gatewayPublic), Gateway: relay.Profile, AuthorityPublic: authority, Cohort: "reference-c2",
		Network: network, Floor: floor, Base: transport}, at)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// runAlphaObserver is an independent, client-only Endpoint process. It shares
// neither the User's broker nor its corpus floor, but resolves the same exact
// alpha link through the same private Relay and Gateway.
func runAlphaObserver(input config) error {
	network, _ := fixed(input.Network)
	authority, err := input.alphaCorpusAuthority()
	if err != nil {
		return err
	}
	endpoint, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: identifier(46), AuthorityPublic: authority,
		IntroductionPublic: make([]byte, ed25519.PublicKeySize), ConnectionPrincipal: identifier(47)})
	if err != nil {
		return err
	}
	defer endpoint.Close()
	at := time.Now().UTC().Truncate(time.Second)
	floor, err := input.openAlphaCorpusFloorAt(input.AlphaObserverCorpusFloorRoot, network)
	if err != nil {
		return err
	}
	defer floor.Close()
	resolver, err := input.openPrivateAlpha(floor, network, at)
	if err != nil {
		return err
	}
	privateBinding, err := endpoint.ResolveAlpha(context.Background(), resolver, input.AlphaServiceLink, at)
	if err != nil {
		return err
	}
	acceptedBinding, err := endpoint.ResolveAcceptedAlpha(floor, input.AlphaServiceLink, at)
	if err != nil {
		return err
	}
	if privateBinding.Link() != acceptedBinding.Link() || privateBinding.Network() != acceptedBinding.Network() ||
		privateBinding.Target() != acceptedBinding.Target() || privateBinding.Serial() != acceptedBinding.Serial() {
		return errors.New("C2 alpha observer exact binding disagrees with its accepted floor")
	}
	return jsonResult("alpha-observer", "resolved")
}

func waitAlphaGatewayReady(path string, deadline time.Time) (alphaGatewayReady, error) {
	var ready alphaGatewayReady
	err := waitAlphaReady(path, deadline, &ready)
	if err != nil || ready.Schema != alphaGatewayReadySchema || ready.URL == "" || ready.ServerName == "" || ready.RootPEM == "" ||
		ready.GatewayPublic == "" || ready.Profile.NetworkID == [32]byte{} || len(ready.Profile.Signature) == 0 {
		return alphaGatewayReady{}, errors.New("C2 alpha Gateway readiness is unavailable")
	}
	return ready, nil
}

func waitAlphaRelayReady(path string, deadline time.Time) (alphaRelayReady, error) {
	var ready alphaRelayReady
	err := waitAlphaReady(path, deadline, &ready)
	if err != nil || ready.Schema != alphaRelayReadySchema || ready.URL == "" || ready.ServerName == "" || ready.RootPEM == "" ||
		ready.GatewayPublic == "" || ready.Profile.NetworkID == [32]byte{} || len(ready.Profile.Signature) == 0 {
		return alphaRelayReady{}, errors.New("C2 alpha Relay readiness is unavailable")
	}
	return ready, nil
}

func waitAlphaReady(path string, deadline time.Time, target any) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := os.ReadFile(path)
		if err == nil && len(raw) > 0 && len(raw) <= 16<<10 {
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if decoder.Decode(target) == nil {
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			return errors.New("C2 alpha fixture readiness timed out")
		}
		<-ticker.C
	}
}

func writeAlphaReady(path string, value any) error {
	if !filepath.IsAbs(path) {
		return errors.New("C2 alpha fixture readiness path is invalid")
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || len(raw) > 16<<10 {
		return errors.New("C2 alpha fixture readiness is invalid")
	}
	temporary := path + ".next"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}

func alphaClientTransport(encodedRoot, serverName string) (*http.Transport, error) {
	root, err := base64.RawStdEncoding.DecodeString(encodedRoot)
	if err != nil {
		return nil, errors.New("C2 alpha fixture TLS root is invalid")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(root) {
		return nil, errors.New("C2 alpha fixture TLS root is invalid")
	}
	return &http.Transport{Proxy: nil, DisableCompression: true, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, ServerName: serverName}}, nil
}

func openFixtureAlphaServer(handler http.Handler, name, listenAddress string) (*http.Server, <-chan error, string, string, []byte, error) {
	certificate, root, err := fixtureAlphaCertificate(name)
	if err != nil {
		return nil, nil, "", "", nil, err
	}
	if listenAddress == "" {
		listenAddress = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return nil, nil, "", "", nil, err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second, IdleTimeout: 5 * time.Second}
	serve := make(chan error, 1)
	go func() {
		serve <- server.Serve(tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}}))
	}()
	return server, serve, listener.Addr().String(), name, root, nil
}

func fixtureAlphaCertificate(name string) (tls.Certificate, []byte, error) {
	rootPublic, rootPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	now := time.Now().UTC()
	rootTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "reference-c2 alpha test root"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	rootRaw, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, rootPublic, rootPrivate)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	leafPublic, leafPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	leafRaw, err := x509.CreateCertificate(rand.Reader, leafTemplate, rootTemplate, leafPublic, rootPrivate)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootRaw})
	return tls.Certificate{Certificate: [][]byte{leafRaw}, PrivateKey: leafPrivate}, rootPEM, nil
}

func closeFixtureAlphaServer(input config, server *http.Server, serve <-chan error, role string) error {
	deadline, _ := input.deadline()
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := waitForTransitCompletion(ctx, input.CompletePath); err != nil {
		_ = server.Close()
		return err
	}
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	if err := <-serve; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return jsonResult(role, "drained")
}
