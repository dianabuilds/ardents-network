package nameresolution_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameadmission"
	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/namestore"
)

func TestDeepestLegalNameResolvesThroughSeparateRoles(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	network := [32]byte{19}
	authoritySeed := sha256.Sum256([]byte("deep-process-name-authority"))
	authority := ed25519.NewKeyFromSeed(authoritySeed[:])
	name, signed := deepProcessRecords(t, network, authority, now)
	materialization := testNamespaceFixture(network, "deep-process-namespace")
	storeRoot := t.TempDir()
	store, err := namestore.Open(storeRoot, materialization.policy)
	if err != nil {
		t.Fatal(err)
	}
	materialization.commit(t, store, 1, signed)
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	gatewayPublic, gatewayPrivate := deterministicResolutionKey("deep-process-gateway")
	bootSecret := [32]byte{23}
	gateway := startResolutionRole(t, roleProcessConfig{Role: "gateway", Network: network,
		NodeID: [32]byte{2}, Family: "gateway-family", AssignmentNotAfter: now.Add(time.Minute).UnixNano(),
		MaximumPending: 8, NamingStoreRoot: storeRoot, IdentityKey: gatewayPrivate,
		Now: now.UnixNano(), AdmissionBootSecret: bootSecret,
		EpochAuthorityIDs: policyIDs(materialization.policy), EpochAuthorityKeys: policyKeys(materialization.policy),
		EpochThreshold: materialization.policy.Threshold})
	relay := startResolutionRole(t, roleProcessConfig{Role: "relay", GatewayURL: gateway.ready.URL,
		GatewayCertificate: gateway.ready.Certificate})

	view := resolutionView(t, network, now, relay.ready.URL, gateway.ready.URL, gatewayPublic)
	bindNamespacePolicy(&view, materialization.policy)
	selection := nameresolution.Selection{At: now, Deadline: now.Add(15 * time.Second),
		RelayNodeID: [32]byte{1}, GatewayNodeID: [32]byte{2}, ConnectionRendezvousNodeID: [32]byte{3}}
	admission, err := nameadmission.NewAdmission([32]byte{2}, network, 1, bootSecret)
	if err != nil {
		t.Fatal(err)
	}
	isolation := [32]byte{29}
	digest := testResolutionAdmissionDigest(t, network, name, selection.Deadline.UnixNano())
	selection.AdmissionChallenge, err = admission.Issue(now.UnixMilli(), "resolution", digest,
		isolation, selection.Deadline.UnixMilli(), [16]byte{31})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := nameresolution.Open(view, selection, gateway.ready.Profile, isolation,
		roleTLSTransport(t, relay.ready.Certificate))
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolver.Resolve(context.Background(), name, now)
	if err != nil || result.Record.Name != name || result.Binding.Target != ([32]byte{1}) {
		t.Fatalf("deep resolution=%+v err=%v", result, err)
	}
}

func deepProcessRecords(t *testing.T, network [32]byte, authority ed25519.PrivateKey,
	now time.Time,
) (string, [][]byte) {
	t.Helper()
	encodedAuthority := hex.EncodeToString(authority.Public().(ed25519.PublicKey))
	records := make([][]byte, 127)
	for depth := 1; depth <= len(records); depth++ {
		name := strings.Repeat("a.", depth-1) + "a"
		record := namelease.Record{Name: name, Generation: 1, Revision: 1, Lease: "active",
			Consistency: "current", Recovery: "stable", Authority: encodedAuthority,
			LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix(), Continuity: 1}
		if depth > 1 {
			record.ParentName = strings.Repeat("a.", depth-2) + "a"
			record.ParentGeneration = 1
		}
		if depth == len(records) {
			record.Target = [32]byte{1}
		}
		var err error
		records[depth-1], err = nameauthority.SignRecord(network, record, authority)
		if err != nil {
			t.Fatal(err)
		}
	}
	return strings.Repeat("a.", 126) + "a", records
}

func deterministicResolutionKey(label string) (ed25519.PublicKey, ed25519.PrivateKey) {
	seed := sha256.Sum256([]byte(label))
	private := ed25519.NewKeyFromSeed(seed[:])
	return private.Public().(ed25519.PublicKey), private
}
