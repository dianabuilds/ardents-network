package reachability_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestGatewayProfileCodecPreservesOnlySignedStateFact(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_400_000, 0).UTC()
	network, node := [32]byte{61}, [32]byte{62}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: network, NodeID: node, IdentityKey: private,
		AssignmentNotAfter: now.Add(time.Minute), Store: store, Clock: func() time.Time { return now },
		AuthorizeDescriptor: func(reachability.Descriptor, time.Time) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := reachability.EncodeGatewayProfile(gateway.Profile())
	if err != nil {
		t.Fatal(err)
	}
	profile, err := reachability.DecodeGatewayProfile(raw)
	if err != nil {
		t.Fatal(err)
	}
	var gatewayPublic [32]byte
	copy(gatewayPublic[:], public)
	if err := reachability.VerifyGatewayProfile(profile, network, node, gatewayPublic, now, now.Add(5*time.Second)); err != nil {
		t.Fatalf("VerifyGatewayProfile = %v", err)
	}

	profile.KeyConfig[0] ^= 1
	if err := reachability.VerifyGatewayProfile(profile, network, node, gatewayPublic, now, now.Add(5*time.Second)); err == nil {
		t.Fatal("altered State profile was accepted")
	}
	if err := reachability.VerifyGatewayProfile(gateway.Profile(), network, node, gatewayPublic, now, now.Add(16*time.Second)); err == nil {
		t.Fatal("profile accepted a lookup window above its fixed bound")
	}
}
