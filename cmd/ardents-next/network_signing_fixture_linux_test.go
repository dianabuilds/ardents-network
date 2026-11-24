//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func signingRecordFixture(t *testing.T) (map[string]any, ed25519.PrivateKey, []byte, time.Time) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	p := preparationRecordPlan(key.Public().(ed25519.PublicKey))
	_, message, err := provisioning.PrepareNetworkPublic("prepare-initial-node-record", preparationJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	reference := time.Date(2033, time.May, 22, 5, 30, 0, 0, time.UTC)
	return p, key, message, reference
}

func signingDeclaration(t *testing.T, purpose string, plan any, reference time.Time) []byte {
	t.Helper()
	field := purpose
	if purpose == "node-record" {
		field = "record"
	}
	return preparationJSON(t, map[string]any{"schema": "ardents-initial-" + purpose + "-signing-plan-v1", "reference_time": reference.Format(time.RFC3339), field: plan})
}

func profileSigningDeclaration(t *testing.T, plan any, public ed25519.PublicKey, reference time.Time) []byte {
	t.Helper()
	return preparationJSON(t, map[string]any{"schema": "ardents-closed-profile-signing-plan-v1", "reference_time": reference.Format(time.RFC3339), "state_authority_key": preparationHex(public), "profile": plan})
}

func signingProfileFixture(t *testing.T, now time.Time) (map[string]any, ed25519.PrivateKey, []byte, []byte) {
	t.Helper()
	key := func(n byte) ed25519.PrivateKey { return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{n}, 32)) }
	state, node, admission := key(81), key(82), key(83)
	var permission [32]byte
	copy(permission[:], admission.Public().(ed25519.PublicKey))
	from := now.UTC().Truncate(time.Hour)
	f, err := networkfixture.BuildClosed(networkfixture.ClosedSpec{NetworkID: [32]byte{1}, Seed: [32]byte{13}, IssuanceAuthority: permission, Number: 1, NotBefore: from, NotAfter: from.Add(2 * time.Hour), Authority: state, Nodes: []networkfixture.ClosedNode{{RecordSpec: networkfixture.RecordSpec{NodeID: [32]byte{2}, Generation: 1, ValidFrom: from, ValidUntil: from.Add(2 * time.Hour), Endpoint: "127.0.0.1:4401", Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 3, PrivateKey: node}, RoleDomain: 2, Subrole: 6}}})
	clear(node)
	clear(admission)
	if err != nil {
		t.Fatal(err)
	}
	p, err := closedprofile.Verify(f.Profile, closedprofile.Context{NetworkID: [32]byte{1}, StateGeneration: f.Epoch.Digest, EpochDigest: f.Epoch.Digest, Epoch: 1, Authority: state.Public().(ed25519.PublicKey), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	nodes, keys := []map[string]any{}, []map[string]any{}
	for _, n := range p.Nodes {
		nodes = append(nodes, map[string]any{"node_id": preparationHex(n.NodeID[:]), "record_digest": preparationHex(n.RecordDigest[:]), "role_domain": n.RoleDomain, "subrole": n.Subrole, "duty_generation": n.DutyGeneration})
	}
	for _, k := range p.Keys {
		keys = append(keys, map[string]any{"window_start": time.Unix(int64(k.WindowStart), 0).UTC().Format(time.RFC3339), "class": k.Class, "spki": preparationHex(k.SPKI)})
	}
	plan := map[string]any{"schema": "ardents-closed-profile-plan-v1", "network_id": preparationIdentifier(1), "state_generation": preparationHex(f.Epoch.Digest[:]), "epoch_digest": preparationHex(f.Epoch.Digest[:]), "issuer_node_id": preparationIdentifier(2), "issuance_authority_key": preparationHex(permission[:]), "epoch": 1, "not_before": p.NotBefore.Format(time.RFC3339), "not_after": p.NotAfter.Format(time.RFC3339), "nodes": nodes, "token_keys": keys}
	return plan, state, bytes.Clone(f.Profile[:len(f.Profile)-64]), f.Profile
}

func certificateDeclaration(t *testing.T, record any, reference time.Time) []byte {
	t.Helper()
	return preparationJSON(t, map[string]any{"schema": "ardents-node-certificate-plan-v1", "reference_time": reference.Format(time.RFC3339Nano), "record": record})
}

func signNetworkFixture(ctx context.Context, args []string) (*provisioning.NetworkSigningResult, error) {
	return provisioning.SignNetwork(ctx, args, nil)
}
