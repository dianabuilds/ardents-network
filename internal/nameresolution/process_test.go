package nameresolution_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/nameresolution"
)

const roleProcessConfigEnvironment = "ARDENTS_RESOLUTION_ROLE_CONFIG"

type roleProcessConfig struct {
	Role               string
	Network            [32]byte
	NodeID             [32]byte
	Family             string
	AssignmentNotAfter int64
	MaximumPending     uint16
	SignedChains       [][][]byte
	IdentityKey        []byte
	Now                int64
	GatewayURL         string
	GatewayCertificate []byte
}

type roleProcessReady struct {
	PID         int
	URL         string
	Certificate []byte
	Profile     nameresolution.GatewayProfile
}

func TestResolutionRolesRunInSeparateProcesses(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	network := [32]byte{9}
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	record := namelease.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(authorityPublic), Target: "target-a",
		LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix()}
	signed, err := nameauthority.SignRecord(network, record, authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gateway := startResolutionRole(t, roleProcessConfig{Role: "gateway", Network: network,
		NodeID: [32]byte{2}, Family: "gateway-family", AssignmentNotAfter: now.Add(time.Minute).UnixNano(),
		MaximumPending: 8, SignedChains: [][][]byte{{signed}}, IdentityKey: gatewayPrivate, Now: now.UnixNano()})
	relay := startResolutionRole(t, roleProcessConfig{Role: "relay", GatewayURL: gateway.ready.URL,
		GatewayCertificate: gateway.ready.Certificate})
	if gateway.ready.PID == relay.ready.PID || gateway.ready.PID == os.Getpid() || relay.ready.PID == os.Getpid() {
		t.Fatal("resolution roles did not receive separate process identities")
	}

	view := resolutionView(t, network, now, relay.ready.URL, gateway.ready.URL, gatewayPublic)
	selection := nameresolution.Selection{At: now, Deadline: now.Add(15 * time.Second),
		RelayNodeID: [32]byte{1}, GatewayNodeID: [32]byte{2}, ConnectionRendezvousNodeID: [32]byte{3}}
	resolver, err := nameresolution.Open(view, selection, gateway.ready.Profile, [32]byte{1},
		roleTLSTransport(t, relay.ready.Certificate))
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolver.Resolve(context.Background(), "alice", now)
	if err != nil || result.Record.Target != "target-a" {
		t.Fatalf("process resolution=%+v err=%v", result, err)
	}
}

type runningResolutionRole struct {
	ready roleProcessReady
	input io.WriteCloser
	done  chan error
}

func startResolutionRole(t *testing.T, config roleProcessConfig) runningResolutionRole {
	t.Helper()
	path := filepath.Join(t.TempDir(), "role.json")
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("write role config: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write role config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestResolutionRoleProcess$")
	command.Env = append(os.Environ(), roleProcessConfigEnvironment+"="+path)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	var ready roleProcessReady
	if err := json.NewDecoder(output).Decode(&ready); err != nil {
		t.Fatalf("start %s role: %v", config.Role, err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = input.Close()
		if err := <-done; err != nil && ctx.Err() == nil {
			t.Errorf("%s role exit: %v", config.Role, err)
		}
		cancel()
	})
	return runningResolutionRole{ready: ready, input: input, done: done}
}

func TestResolutionRoleProcess(t *testing.T) {
	path := os.Getenv(roleProcessConfigEnvironment)
	if path == "" {
		t.Skip("helper process only")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config roleProcessConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	var server *httptest.Server
	ready := roleProcessReady{PID: os.Getpid()}
	switch config.Role {
	case "gateway":
		gateway, openErr := nameresolution.NewGateway(nameresolution.GatewayConfig{NetworkID: config.Network,
			NodeID: config.NodeID, Family: config.Family, Domain: "rendezvous",
			AssignmentNotAfter: time.Unix(0, config.AssignmentNotAfter), MaximumPending: config.MaximumPending,
			SignedRecordChains: config.SignedChains, IdentityKey: ed25519.PrivateKey(config.IdentityKey),
			Clock: func() time.Time { return time.Unix(0, config.Now) }})
		if openErr != nil {
			t.Fatal(openErr)
		}
		server = httptest.NewTLSServer(gateway.Handler())
		ready.Profile = gateway.Profile()
	case "relay":
		relay, openErr := nameresolution.NewRelay(config.GatewayURL,
			&http.Client{Transport: roleTLSTransport(t, config.GatewayCertificate)})
		if openErr != nil {
			t.Fatal(openErr)
		}
		server = httptest.NewTLSServer(relay.Handler())
	default:
		t.Fatal("unknown resolution role")
	}
	defer server.Close()
	ready.URL, ready.Certificate = server.URL, server.Certificate().Raw
	if err := json.NewEncoder(os.Stdout).Encode(ready); err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func roleTLSTransport(t *testing.T, certificate []byte) *http.Transport {
	t.Helper()
	parsed, err := x509.ParseCertificate(certificate)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}}
}
