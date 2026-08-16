package route_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestCanaryCrossesEveryRealRoutePosition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	identities := make([]testRouteIdentity, 6)
	for index := range identities {
		identities[index] = routeIdentity(t, byte(index+1))
	}
	addresses := make([]string, 5)
	for index := range addresses {
		addresses[index] = unusedAddress(t)
	}
	plan := route.Plan{NetworkID: [32]byte{8}, Generation: "generation-7", Epoch: 7, Digest: [32]byte{7},
		Profile: "h3-route-tracer-v1", ViewRoot: [32]byte{6}, Seed: [32]byte{5}, SelectionAt: time.Now().Unix()}
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	for index, role := range roles {
		plan.Positions = append(plan.Positions, route.Position{Role: role, Domain: role,
			NodeID: [32]byte{byte(index + 1)}, PublicKey: identities[index].public,
			Family: "family-" + role, Endpoint: addresses[index], Capacity: 1})
	}
	ready := make(chan route.Evidence, 5)
	done := make(chan route.Evidence, 5)
	start := func(config route.Actor) {
		go func() {
			observation, err := route.Run(ctx, config, func(value route.Evidence) { ready <- value })
			if err != nil {
				observation.Error = err.Error()
			}
			done <- observation
		}()
	}
	for index := 3; index >= 0; index-- {
		nextAddress, nextID, nextPin := addresses[4], [32]byte{90}, identities[4].public
		if index < 3 {
			nextAddress, nextID, nextPin = addresses[index+1], plan.Positions[index+1].NodeID, identities[index+1].public
		}
		upstream := identities[5].public
		if index > 0 {
			upstream = identities[index-1].public
		}
		start(route.Actor{ManifestDigest: [32]byte{99}, NetworkID: plan.NetworkID, EpochDigest: plan.Digest,
			Role: roles[index], NodeID: plan.Positions[index].NodeID, ListenAddress: addresses[index],
			Certificate: identities[index].certificate, UpstreamPin: upstream,
			NextNodeID: nextID, NextAddress: nextAddress, NextPin: nextPin, Deadline: 5 * time.Second})
	}
	go func() {
		observation, err := route.Run(ctx, route.Actor{Role: "publisher", ManifestDigest: [32]byte{99}, NetworkID: plan.NetworkID,
			EpochDigest: plan.Digest, NodeID: [32]byte{90},
			ListenAddress: addresses[4], Certificate: identities[4].certificate,
			UpstreamPin: identities[3].public, ServiceCertificate: identities[4].certificate,
			Deadline: 5 * time.Second}, func(value route.Evidence) { ready <- value })
		if err != nil {
			observation.Error = err.Error()
		}
		done <- observation
	}()
	for range 5 {
		select {
		case value := <-ready:
			if value.Kind != "ready" {
				t.Fatalf("unexpected readiness observation: %+v", value)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	result, err := route.Run(ctx, route.Actor{Role: "client", ManifestDigest: [32]byte{99}, Plan: plan,
		ClientCertificate: identities[5].certificate, PublisherPin: identities[4].public,
		LocalRoleStateRoot: t.TempDir(), Deadline: 5 * time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanaryLength != 32 || result.CanaryDigest != sha256.Sum256(result.Canary) || len(result.Canary) != 32 {
		t.Fatalf("canary result does not match: %+v", result)
	}
	observations := make([]route.Evidence, 0, 5)
	for range 5 {
		select {
		case value := <-done:
			if value.Error != "" {
				t.Fatalf("route process failed: %+v", value)
			}
			observations = append(observations, value)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for _, observation := range observations {
		if observation.Kind != "complete" || observation.PID == 0 {
			t.Fatalf("incomplete process observation: %+v", observation)
		}
		if observation.Role != "publisher" && observation.OpaqueBytes == 0 {
			t.Fatalf("role forwarded no opaque bytes: %+v", observation)
		}
		if observation.Role != "publisher" && observation.CanaryDigest == result.CanaryDigest {
			t.Fatalf("role observed plaintext canary digest: %+v", observation)
		}
	}
}

func TestRoleProcessesServeConcurrentAttachments(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	identities := make([]testRouteIdentity, 6)
	for index := range identities {
		identities[index] = routeIdentity(t, byte(index+31))
	}
	addresses := make([]string, 5)
	for index := range addresses {
		addresses[index] = unusedAddress(t)
	}
	plan := route.Plan{NetworkID: [32]byte{18}, Generation: "capacity-generation", Epoch: 9, Digest: [32]byte{19},
		Profile: "h3-route-tracer-v1", ViewRoot: [32]byte{20}, Seed: [32]byte{21}, SelectionAt: time.Now().Unix()}
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	for index, role := range roles {
		plan.Positions = append(plan.Positions, route.Position{Role: role, Domain: role,
			NodeID: [32]byte{byte(index + 31)}, PublicKey: identities[index].public,
			Family: "capacity-" + role, Endpoint: addresses[index], Capacity: 2})
	}
	ready, done := make(chan route.Evidence, 5), make(chan route.Evidence, 5)
	start := func(actor route.Actor) {
		go func() {
			observation, err := route.Run(ctx, actor, func(value route.Evidence) { ready <- value })
			if err != nil {
				observation.Error = err.Error()
			}
			done <- observation
		}()
	}
	for index := 3; index >= 0; index-- {
		nextAddress, nextID, nextPin := addresses[4], [32]byte{90}, identities[4].public
		if index < 3 {
			nextAddress, nextID, nextPin = addresses[index+1], plan.Positions[index+1].NodeID, identities[index+1].public
		}
		upstream := identities[5].public
		if index > 0 {
			upstream = identities[index-1].public
		}
		start(route.Actor{ManifestDigest: [32]byte{99}, NetworkID: plan.NetworkID, EpochDigest: plan.Digest,
			Role: roles[index], NodeID: plan.Positions[index].NodeID, ListenAddress: addresses[index],
			Certificate: identities[index].certificate, UpstreamPin: upstream, NextNodeID: nextID,
			NextAddress: nextAddress, NextPin: nextPin, MaximumAttachments: 2, Deadline: 5 * time.Second})
	}
	start(route.Actor{Role: "publisher", ManifestDigest: [32]byte{99}, NetworkID: plan.NetworkID,
		EpochDigest: plan.Digest, NodeID: [32]byte{90}, ListenAddress: addresses[4],
		Certificate: identities[4].certificate, UpstreamPin: identities[3].public,
		ServiceCertificate: identities[4].certificate, MaximumAttachments: 2, Deadline: 5 * time.Second})
	for range 5 {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for _, address := range addresses {
		connection, err := net.Dial("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = connection.Write([]byte("not-a-TLS-setup"))
		_ = connection.Close()
	}
	time.Sleep(25 * time.Millisecond)
	clients := make(chan error, 2)
	localRoleRoots := [2]string{t.TempDir(), t.TempDir()}
	for index := range 2 {
		go func(localRoleRoot string) {
			result, err := route.Run(ctx, route.Actor{Role: "client", ManifestDigest: [32]byte{99}, Plan: plan,
				ClientCertificate: identities[5].certificate, PublisherPin: identities[4].public,
				LocalRoleStateRoot: localRoleRoot, Deadline: 5 * time.Second}, nil)
			if err == nil && (result.CanaryLength != 32 || len(result.Canary) != 32) {
				err = errors.New("capacity client received an invalid canary result")
			}
			clients <- err
		}(localRoleRoots[index])
	}
	for range 2 {
		if err := <-clients; err != nil {
			t.Fatal(err)
		}
	}
	for range 5 {
		observation := <-done
		if observation.Error != "" || observation.AttachmentsCompleted != 2 || observation.AttachmentsAbandoned != 1 {
			t.Fatalf("bounded role capacity failed: %+v", observation)
		}
	}
}

func TestRouteCapacityProtectsEstablishedAttachmentThenDrains(t *testing.T) {
	upstream, node, downstream := routeIdentity(t, 71), routeIdentity(t, 72), routeIdentity(t, 73)
	nodeAddress, nextAddress := unusedAddress(t), unusedAddress(t)
	nextListener, err := net.Listen("tcp", nextAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer nextListener.Close()
	downstreamReceived := make(chan byte, 1)
	go func() {
		connection, acceptErr := nextListener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		secured := tls.Server(connection, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{downstream.certificate}, ClientAuth: tls.RequireAnyClientCert})
		if secured.Handshake() != nil {
			return
		}
		frame := make([]byte, 101)
		if _, readErr := io.ReadFull(secured, frame); readErr != nil {
			return
		}
		if _, writeErr := secured.Write([]byte("ARLA")); writeErr != nil {
			return
		}
		one := make([]byte, 1)
		if _, readErr := io.ReadFull(secured, one); readErr == nil {
			downstreamReceived <- one[0]
			_, _ = io.Copy(io.Discard, secured)
		}
	}()
	var high, memory atomic.Uint64
	events := make(chan route.Evidence, 8)
	done := make(chan error, 1)
	go func() {
		_, runErr := route.Run(context.Background(), route.Actor{Role: "initiator", ManifestDigest: [32]byte{99},
			NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3}, ListenAddress: nodeAddress,
			Certificate: node.certificate, UpstreamPin: upstream.public, NextNodeID: [32]byte{4},
			NextAddress: nextAddress, NextPin: downstream.public, Deadline: 2 * time.Second, Lifetime: 2 * time.Second,
			MaximumAttachments: 2, AttachmentTarget: 1, ResourceProfile: "h3-np1-v1",
			ResourceCheck:    func() error { return nil },
			PressureInterval: 5 * time.Millisecond, ResourceMeasure: func() (resource.Sample, error) {
				return resource.Sample{HighEvents: high.Load(), MemoryBytes: memory.Load()}, nil
			}}, func(value route.Evidence) { events <- value })
		done <- runErr
	}()
	if event := <-events; event.Kind != "ready" || event.State != "NORMAL" {
		t.Fatalf("initial capacity state = %+v", event)
	}
	connection, err := tls.Dial("tcp", nodeAddress, &tls.Config{MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true, Certificates: []tls.Certificate{upstream.certificate}})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := testLegBinding(connection, [32]byte{1}, [32]byte{2}, [32]byte{3}); err != nil {
		t.Fatal(err)
	}
	slowSetup, err := net.Dial("tcp", nodeAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer slowSetup.Close()
	high.Store(1)
	if event := <-events; event.State != "PROTECT" {
		t.Fatalf("capacity did not enter PROTECT: %+v", event)
	}
	refused, err := net.Dial("tcp", nodeAddress)
	if err == nil {
		defer refused.Close()
		_ = refused.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		if _, err = refused.Read(make([]byte, 1)); err == nil {
			t.Fatal("PROTECT accepted new setup work")
		}
	}
	if _, err := connection.Write([]byte{0x5a}); err != nil {
		t.Fatalf("PROTECT interrupted established Attachment: %v", err)
	}
	select {
	case value := <-downstreamReceived:
		if value != 0x5a {
			t.Fatalf("established Attachment changed byte: %x", value)
		}
	case <-time.After(time.Second):
		t.Fatal("established Attachment made no progress in PROTECT")
	}
	memory.Store(460 << 20)
	if event := <-events; event.State != "DRAIN" {
		t.Fatalf("capacity did not enter DRAIN: %+v", event)
	}
	if event := <-events; event.State != "EXIT" {
		t.Fatalf("capacity did not enter EXIT: %+v", event)
	}
	if err := <-done; err == nil {
		t.Fatal("emergency Route drain returned success")
	}
	_ = slowSetup.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := slowSetup.Read(make([]byte, 1)); err == nil {
		t.Fatal("EXIT retained incomplete setup work")
	}
	if listener, err := net.Listen("tcp", nodeAddress); err != nil {
		t.Fatalf("EXIT retained Route listener: %v", err)
	} else {
		listener.Close()
	}
}

func TestActorsRejectCrossRoleInformation(t *testing.T) {
	identity, upstream := routeIdentity(t, 51), routeIdentity(t, 52)
	plan := route.Plan{NetworkID: [32]byte{1}, Generation: "generation", Epoch: 1, Digest: [32]byte{2},
		Profile: "h3-route-tracer-v1", ViewRoot: [32]byte{6}, Seed: [32]byte{5}, SelectionAt: time.Now().Unix()}
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	for index, role := range roles {
		plan.Positions = append(plan.Positions, route.Position{Role: role, Domain: role, NodeID: [32]byte{byte(index + 3)},
			PublicKey: [32]byte{byte(index + 10)}, Family: role + "-family", Endpoint: unusedAddress(t), Capacity: 1})
	}
	tests := []struct {
		name  string
		actor route.Actor
	}{
		{"Node full Plan", route.Actor{Role: "initiator", ManifestDigest: [32]byte{99}, NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3},
			ListenAddress: unusedAddress(t), Certificate: identity.certificate, UpstreamPin: upstream.public,
			NextNodeID: [32]byte{4}, NextAddress: unusedAddress(t), NextPin: upstream.public, Plan: plan, Deadline: time.Second}},
		{"Publisher next hop", route.Actor{Role: "publisher", ManifestDigest: [32]byte{99}, NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3},
			ListenAddress: unusedAddress(t), Certificate: identity.certificate, UpstreamPin: upstream.public,
			ServiceCertificate: identity.certificate, NextNodeID: [32]byte{4}, Deadline: time.Second}},
		{"Client listener", route.Actor{Role: "client", ManifestDigest: [32]byte{99}, Plan: plan, ClientCertificate: identity.certificate,
			PublisherPin: upstream.public, ListenAddress: unusedAddress(t), LocalRoleStateRoot: t.TempDir(), Deadline: time.Second}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := route.Run(context.Background(), test.actor, nil); err == nil || !strings.Contains(err.Error(), "role-local") {
				t.Fatalf("cross-role information was not rejected: %v", err)
			}
		})
	}
}

type testRouteIdentity struct {
	public      [32]byte
	certificate tls.Certificate
}

func routeIdentity(t *testing.T, marker byte) testRouteIdentity {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	template := &x509.Certificate{SerialNumber: big.NewInt(int64(marker)), Subject: pkix.Name{CommonName: "route.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(nil, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	var fixed [32]byte
	copy(fixed[:], public)
	return testRouteIdentity{public: fixed, certificate: tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: parsed}}
}

func unusedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}
