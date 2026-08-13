package route_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

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
	plan := route.Plan{NetworkID: [32]byte{8}, Generation: "generation-7", Epoch: 7, Digest: [32]byte{7}}
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
		start(route.Actor{NetworkID: plan.NetworkID, EpochDigest: plan.Digest,
			Role: roles[index], NodeID: plan.Positions[index].NodeID, ListenAddress: addresses[index],
			Certificate: identities[index].certificate, UpstreamPin: upstream,
			NextNodeID: nextID, NextAddress: nextAddress, NextPin: nextPin, Deadline: 5 * time.Second})
	}
	go func() {
		observation, err := route.Run(ctx, route.Actor{Role: "publisher", NetworkID: plan.NetworkID,
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
	canary := bytes.Repeat([]byte{0xa7}, 32)
	result, err := route.Run(ctx, route.Actor{Role: "client", Plan: plan,
		ClientCertificate: identities[5].certificate, PublisherPin: identities[4].public,
		Canary: canary, Deadline: 5 * time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanaryLength != 32 || result.CanaryDigest != sha256.Sum256(canary) || !bytes.Equal(result.Canary, canary) {
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

func TestActorsRejectCrossRoleInformation(t *testing.T) {
	identity, upstream := routeIdentity(t, 51), routeIdentity(t, 52)
	plan := route.Plan{NetworkID: [32]byte{1}, Generation: "generation", Epoch: 1, Digest: [32]byte{2}}
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	for index, role := range roles {
		plan.Positions = append(plan.Positions, route.Position{Role: role, Domain: role, NodeID: [32]byte{byte(index + 3)},
			PublicKey: [32]byte{byte(index + 10)}, Family: role + "-family", Endpoint: unusedAddress(t), Capacity: 1})
	}
	tests := []struct {
		name  string
		actor route.Actor
	}{
		{"Node full Plan", route.Actor{Role: "initiator", NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3},
			ListenAddress: unusedAddress(t), Certificate: identity.certificate, UpstreamPin: upstream.public,
			NextNodeID: [32]byte{4}, NextAddress: unusedAddress(t), NextPin: upstream.public, Plan: plan, Deadline: time.Second}},
		{"Publisher next hop", route.Actor{Role: "publisher", NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3},
			ListenAddress: unusedAddress(t), Certificate: identity.certificate, UpstreamPin: upstream.public,
			ServiceCertificate: identity.certificate, NextNodeID: [32]byte{4}, Deadline: time.Second}},
		{"Client listener", route.Actor{Role: "client", Plan: plan, ClientCertificate: identity.certificate,
			PublisherPin: upstream.public, ListenAddress: unusedAddress(t), Deadline: time.Second}},
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
