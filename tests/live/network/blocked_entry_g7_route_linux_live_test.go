//go:build linux && live

package network_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestG7RouteEntryFailureHasNoAlternateDial(t *testing.T) {
	variant := os.Getenv("ARDENTS_G7_VARIANT")
	if variant != "ordinary-entry" && variant != "direct-target" && variant != "shorter-route" {
		t.Skip("selected G7 Route fallback variants only")
	}
	listeners := make([]net.Listener, 5)
	for index := range listeners {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners[index] = listener
		defer listener.Close()
		proveG7ListenerReachable(t, listener)
	}
	now := time.Now()
	certificatePEM, keyPEM, _ := issueBlockedCertificate(t, now, "g7-client", nil,
		0, nil)
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	plan := route.Plan{NetworkID: [32]byte{1}, Generation: "g7", Epoch: 1, Digest: [32]byte{2},
		Profile: "h3-route-tracer-v1", ViewRoot: [32]byte{3}, Seed: [32]byte{4}, SelectionAt: now.Unix()}
	for index, role := range roles {
		plan.Positions = append(plan.Positions, route.Position{Role: role, Domain: role,
			NodeID: [32]byte{byte(index + 10)}, PublicKey: [32]byte{byte(index + 20)},
			Family: fmt.Sprintf("g7-family-%d", index), Endpoint: listeners[index].Addr().String(), Capacity: 1})
	}
	stream, peer := net.Pipe()
	defer stream.Close()
	defer peer.Close()
	input := route.Actor{ManifestDigest: [32]byte{5}, Role: "client", Plan: plan,
		ClientCertificate: certificate, RawAttachment: true, Stream: stream,
		Deadline: time.Second, Lifetime: time.Second, LocalRoleStateRoot: t.TempDir(),
		OpenEntry: func(context.Context, func(context.Context, net.Conn) (*tls.Conn, error)) (*tls.Conn, func() error, error) {
			return nil, nil, errors.New("bridge-attempt-exhausted")
		}}
	if _, err := route.Run(context.Background(), input, nil); err == nil {
		t.Fatal("failed Bridge entry unexpectedly returned success")
	}
	for _, listener := range listeners {
		assertG7ListenerUnused(t, listener)
	}
	rawInput, err := json.Marshal(struct {
		Variant       string     `json:"variant"`
		Plan          route.Plan `json:"plan"`
		DirectAddress string     `json:"direct_address"`
	}{variant, plan, listeners[4].Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	component := map[string]string{"ordinary-entry": "route-entry", "direct-target": "endpoint-route",
		"shorter-route": "route-plan"}[variant]
	contract, _ := json.Marshal(g7ComponentContract{Schema: "ardents-h3-g7-component-v1", Variant: variant,
		Component: component, Input: rawInput,
		ReachableTargets: []string{listeners[0].Addr().String(), listeners[1].Addr().String(),
			listeners[2].Addr().String(), listeners[3].Addr().String(), listeners[4].Addr().String()},
		ObservedTargets: []string{}, EntryError: "bridge-attempt-exhausted"})
	fmt.Printf("g7-component-contract=%s\n", contract)
}

func proveG7ListenerReachable(t *testing.T, listener net.Listener) {
	t.Helper()
	accepted := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			_ = connection.Close()
		}
		accepted <- err
	}()
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
}

func assertG7ListenerUnused(t *testing.T, listener net.Listener) {
	t.Helper()
	if tcp, ok := listener.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(20 * time.Millisecond))
	}
	connection, err := listener.Accept()
	if err == nil {
		_ = connection.Close()
		t.Fatal("Route used a forbidden reachable path after Bridge failure")
	}
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatal(err)
	}
}
