package source

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestInspectLatestUsesOneHonestDeclarationAndGenuineMTLS(t *testing.T) {
	now := time.Now()
	fixture := newSourceTLSFixture(t, now)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x99}, ed25519.SeedSize))
	defer clear(authority)
	public := authority.Public().(ed25519.PublicKey)
	pins := map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}
	clock := func() time.Time { return now }
	serving, _, err := New(Config{ServeAddress: address, ServeCertificate: fixture.server, ServeClientRootPEM: fixture.authority.rootPEM,
		ServeClientKeyDigests: [][32]byte{fixture.clientPin}, VerificationClock: clock}, pins)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	ready, joined := make(chan error, 1), make(chan error, 1)
	var requests atomic.Int32
	network := [32]byte{11}
	go func() {
		joined <- serving.Serve(ctx, ready, nil, nil, func(_ context.Context, request Message) Message {
			requests.Add(1)
			if request.Operation != "latest" || request.NetworkDigest != NetworkDigest(network) || request.MaterialIndex != 3 {
				t.Error("different request reached genuine transport", request)
			}
			return Message{Status: "not-found"}
		})
	}()
	defer func() {
		cancel()
		if err := <-joined; err != context.Canceled {
			t.Error("serving retirement", err)
		}
	}()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	input := Inspection{Source: Source{Address: address, ServerName: fixture.serverName, Identity: [32]byte{7}, Family: "one-effective-controller",
		EndpointHandle: "one-source", RootPEM: fixture.authority.rootPEM, LeafKeyDigest: fixture.serverPin}, ClientCertificate: fixture.client,
		Authorities: pins, VerificationClock: clock, NetworkID: network, MaterialIndex: 3}
	wrong := input
	wrong.Source.LeafKeyDigest = [32]byte{1}
	if _, err := InspectLatest(ctx, wrong); !errors.Is(err, ErrAuthentication) {
		t.Fatal("wrong Source key pin reached inspection", err)
	}
	if requests.Load() != 0 {
		t.Fatal("failed authentication reached resolver")
	}
	response, err := InspectLatest(ctx, input)
	if err != nil || response.Status != "not-found" || requests.Load() != 1 {
		t.Fatal("single genuine inspection", response, err, requests.Load())
	}
	// A one-contact inspection never loosens the actual two-source acquisition
	// rule, even when called with complete real client credentials.
	if _, _, err := New(Config{Sources: [2]Source{input.Source}, ClientCertificate: fixture.client, VerificationClock: clock}, pins); err == nil {
		t.Fatal("one-contact inspection widened acquisition policy")
	}
	canceled, stop := context.WithCancelCause(ctx)
	cause := errors.New("original inspection caller retired")
	stop(cause)
	if response, err := InspectLatest(canceled, input); !errors.Is(err, cause) || !errors.Is(err, context.Canceled) || response.Status != "" {
		t.Fatal("inspection lost original caller", response, err)
	}
	if requests.Load() != 1 {
		t.Fatal("canceled inspection retried the Source")
	}
}
