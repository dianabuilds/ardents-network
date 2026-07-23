package trust_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"reflect"
	"sync"
	"testing"

	"ardents/internal/identity/principal"
	"ardents/internal/identity/trust"
)

func TestRegistryScopesTrustByExactPurpose(t *testing.T) {
	publicKey := publicKeyForSeedByte(t, 1)
	trustedPrincipal, err := principal.FromEd25519PublicKey(publicKey)
	if err != nil {
		t.Fatalf("derive principal: %v", err)
	}

	registry, err := trust.NewRegistry([]trust.Entry{{
		Principal: trustedPrincipal.String(),
		PublicKey: publicKey,
		Purposes:  []trust.Purpose{trust.PurposeDiscoveryPublish},
	}})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	got, ok := registry.Lookup(trust.PurposeDiscoveryPublish, trustedPrincipal)
	if !ok || !bytes.Equal(got, publicKey) {
		t.Fatalf("discovery lookup = (%x, %v), want trusted public key", got, ok)
	}

	for _, purpose := range []trust.Purpose{
		trust.PurposeChannelIssue,
		trust.PurposeIdentityAttest,
		trust.Purpose("unknown"),
		"",
	} {
		if got, ok := registry.Lookup(purpose, trustedPrincipal); ok || got != nil {
			t.Fatalf("lookup(%q) = (%x, %v), want fail-closed denial", purpose, got, ok)
		}
	}
}

func TestRegistryRejectsInvalidAndAmbiguousDefinitions(t *testing.T) {
	firstKey := publicKeyForSeedByte(t, 1)
	firstPrincipal, err := principal.FromEd25519PublicKey(firstKey)
	if err != nil {
		t.Fatalf("derive first principal: %v", err)
	}
	secondKey := publicKeyForSeedByte(t, 2)

	valid := trust.Entry{
		Principal: firstPrincipal.String(),
		PublicKey: firstKey,
		Purposes:  []trust.Purpose{trust.PurposeDiscoveryPublish},
	}
	tests := map[string][]trust.Entry{
		"empty Principal": {{PublicKey: firstKey, Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish}}},
		"malformed Principal": {{
			Principal: "p1_bad", PublicKey: firstKey, Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish},
		}},
		"short public key": {{
			Principal: firstPrincipal.String(), PublicKey: ed25519.PublicKey{1}, Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish},
		}},
		"mismatched public key": {{
			Principal: firstPrincipal.String(), PublicKey: secondKey, Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish},
		}},
		"no purpose": {{Principal: firstPrincipal.String(), PublicKey: firstKey}},
		"empty purpose": {{
			Principal: firstPrincipal.String(), PublicKey: firstKey, Purposes: []trust.Purpose{""},
		}},
		"unknown purpose": {{
			Principal: firstPrincipal.String(), PublicKey: firstKey, Purposes: []trust.Purpose{"realm.admin"},
		}},
		"duplicate purpose": {{
			Principal: firstPrincipal.String(), PublicKey: firstKey,
			Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish, trust.PurposeDiscoveryPublish},
		}},
		"duplicate definition": {valid, valid},
		"ambiguous split definition": {
			valid,
			{Principal: firstPrincipal.String(), PublicKey: firstKey, Purposes: []trust.Purpose{trust.PurposeChannelIssue}},
		},
	}

	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := trust.NewRegistry(entries); err == nil {
				t.Fatal("NewRegistry() error = nil, want fail-closed rejection")
			}
		})
	}
}

func TestRegistrySnapshotAndGenerationAreCanonicalAndDetached(t *testing.T) {
	firstKey := publicKeyForSeedByte(t, 3)
	firstPrincipal, err := principal.FromEd25519PublicKey(firstKey)
	if err != nil {
		t.Fatalf("derive first principal: %v", err)
	}
	secondKey := publicKeyForSeedByte(t, 4)
	secondPrincipal, err := principal.FromEd25519PublicKey(secondKey)
	if err != nil {
		t.Fatalf("derive second principal: %v", err)
	}

	definitions := []trust.Entry{
		{
			Principal: secondPrincipal.String(), PublicKey: secondKey,
			Purposes: []trust.Purpose{trust.PurposeIdentityAttest, trust.PurposeChannelIssue},
		},
		{
			Principal: firstPrincipal.String(), PublicKey: firstKey,
			Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish},
		},
	}
	registry, err := trust.NewRegistry(definitions)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	reordered, err := trust.NewRegistry([]trust.Entry{
		definitions[1],
		{
			Principal: definitions[0].Principal, PublicKey: definitions[0].PublicKey,
			Purposes: []trust.Purpose{trust.PurposeChannelIssue, trust.PurposeIdentityAttest},
		},
	})
	if err != nil {
		t.Fatalf("new reordered registry: %v", err)
	}

	snapshot := registry.Snapshot()
	if !reflect.DeepEqual(snapshot, reordered.Snapshot()) {
		t.Fatalf("canonical snapshots differ:\nfirst:  %#v\nsecond: %#v", snapshot, reordered.Snapshot())
	}
	if snapshot.Generation != registry.Generation() {
		t.Fatal("snapshot and registry generations differ")
	}
	encodedGeneration := snapshot.Generation.String()
	if len(encodedGeneration) != 64 {
		t.Fatalf("generation length = %d, want full 64-character SHA-256", len(encodedGeneration))
	}
	if _, err := hex.DecodeString(encodedGeneration); err != nil {
		t.Fatalf("generation is not hexadecimal: %v", err)
	}
	if len(snapshot.Entries) != 2 || snapshot.Entries[0].Principal > snapshot.Entries[1].Principal {
		t.Fatalf("snapshot entries are not canonically sorted: %#v", snapshot.Entries)
	}
	for _, entry := range snapshot.Entries {
		for i := 1; i < len(entry.Purposes); i++ {
			if entry.Purposes[i-1] > entry.Purposes[i] {
				t.Fatalf("snapshot purposes are not canonically sorted: %#v", entry.Purposes)
			}
		}
	}

	wantKey := append(ed25519.PublicKey(nil), firstKey...)
	definitions[1].PublicKey[0] ^= 0xff
	definitions[1].Purposes[0] = trust.PurposeChannelIssue
	lookedUp, ok := registry.Lookup(trust.PurposeDiscoveryPublish, firstPrincipal)
	if !ok || !bytes.Equal(lookedUp, wantKey) {
		t.Fatal("registry retained mutable constructor input")
	}
	lookedUp[0] ^= 0xff
	if again, ok := registry.Lookup(trust.PurposeDiscoveryPublish, firstPrincipal); !ok || !bytes.Equal(again, wantKey) {
		t.Fatal("lookup returned registry-owned key memory")
	}
	snapshot.Entries[0].PublicKey[0] ^= 0xff
	snapshot.Entries[0].Purposes[0] = trust.PurposeIdentityAttest
	if reflect.DeepEqual(snapshot, registry.Snapshot()) {
		t.Fatal("snapshot mutation unexpectedly changed no data")
	}
	if !reflect.DeepEqual(registry.Snapshot(), reordered.Snapshot()) {
		t.Fatal("snapshot returned registry-owned memory")
	}

	rotatedKey := publicKeyForSeedByte(t, 5)
	rotatedPrincipal, err := principal.FromEd25519PublicKey(rotatedKey)
	if err != nil {
		t.Fatalf("derive rotated principal: %v", err)
	}
	rotated, err := trust.NewRegistry([]trust.Entry{{
		Principal: rotatedPrincipal.String(), PublicKey: rotatedKey,
		Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish},
	}})
	if err != nil {
		t.Fatalf("new rotated registry: %v", err)
	}
	beforeRotation, err := trust.NewRegistry([]trust.Entry{{
		Principal: firstPrincipal.String(), PublicKey: wantKey,
		Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish},
	}})
	if err != nil {
		t.Fatalf("new pre-rotation registry: %v", err)
	}
	changedPurpose, err := trust.NewRegistry([]trust.Entry{{
		Principal: firstPrincipal.String(), PublicKey: wantKey,
		Purposes: []trust.Purpose{trust.PurposeChannelIssue},
	}})
	if err != nil {
		t.Fatalf("new purpose-rotated registry: %v", err)
	}
	if beforeRotation.Generation() == rotated.Generation() {
		t.Fatal("key/Principal rotation did not change generation")
	}
	if beforeRotation.Generation() == changedPurpose.Generation() {
		t.Fatal("purpose rotation did not change generation")
	}
}

func TestRegistrySupportsConcurrentDetachedReads(t *testing.T) {
	publicKey := publicKeyForSeedByte(t, 6)
	trustedPrincipal, err := principal.FromEd25519PublicKey(publicKey)
	if err != nil {
		t.Fatalf("derive principal: %v", err)
	}
	registry, err := trust.NewRegistry([]trust.Entry{{
		Principal: trustedPrincipal.String(), PublicKey: publicKey,
		Purposes: []trust.Purpose{trust.PurposeDiscoveryPublish},
	}})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	var workers sync.WaitGroup
	failures := make(chan string, 32)
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 100 {
				key, ok := registry.Lookup(trust.PurposeDiscoveryPublish, trustedPrincipal)
				if !ok || !bytes.Equal(key, publicKey) {
					failures <- "lookup changed"
					return
				}
				key[0] ^= 0xff
				snapshot := registry.Snapshot()
				if len(snapshot.Entries) != 1 {
					failures <- "snapshot changed"
					return
				}
				snapshot.Entries[0].PublicKey[0] ^= 0xff
			}
		}()
	}
	workers.Wait()
	close(failures)
	for failure := range failures {
		t.Fatal(failure)
	}
}

func publicKeyForSeedByte(t *testing.T, value byte) ed25519.PublicKey {
	t.Helper()
	seed := bytes.Repeat([]byte{value}, ed25519.SeedSize)
	privateKey := ed25519.NewKeyFromSeed(seed)
	return append(ed25519.PublicKey(nil), privateKey.Public().(ed25519.PublicKey)...)
}
