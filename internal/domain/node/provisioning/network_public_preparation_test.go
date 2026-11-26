package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/closedprofile"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func publicEpochFixture(t *testing.T) (map[string]any, ed25519.PrivateKey, [][]byte, networkfixture.Epoch, time.Time) {
	t.Helper()
	from, _ := preparationTime("2033-05-22T05:20:00Z")
	until := from.Add(time.Hour)
	reference := from.Add(time.Minute)
	stateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, 32))
	var records []networkfixture.Record
	var inputs [][]byte
	for _, n := range []byte{5, 2} {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{n}, 32))
		family := "family-two"
		if n == 5 {
			family = "family-five"
		}
		r, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: [32]byte{1}, NodeID: [32]byte{n}, Generation: 1, ValidFrom: from, ValidUntil: until, Family: family, Endpoint: fmt.Sprintf("127.0.0.1:%d", 4400+int(n)), Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 3, PrivateKey: key})
		clear(key)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, r)
		inputs = append(inputs, r.Raw)
	}
	domains := []string{"initiator", "introduction", "rendezvous", "responder"}
	f, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{NetworkID: [32]byte{1}, Number: 1, ValidFrom: from, ValidUntil: until, Inputs: inputs, Accepted: records, AssignmentSeed: [32]byte{13}, Domains: domains, Authorities: []ed25519.PrivateKey{stateKey}, Profile: "ardents-route-v3", Version: 3})
	if err != nil {
		t.Fatal(err)
	}
	p := map[string]any{"schema": "ardents-initial-epoch-plan-v1", "network_id": preparationIdentifier(1), "assignment_seed": preparationIdentifier(13), "valid_from": from.Format(time.RFC3339), "valid_until": until.Format(time.RFC3339), "domains": domains, "records": []string{preparationHex(inputs[0]), preparationHex(inputs[1])}, "authority_keys": []string{preparationHex(stateKey.Public().(ed25519.PublicKey))}}
	return p, stateKey, inputs, f, reference
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

func TestPublicNetworkPreparationPortable(t *testing.T) {
	recordKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	defer clear(recordKey)
	recordPlan := preparationRecordPlan(recordKey.Public().(ed25519.PublicKey))
	name, unsigned, err := PrepareNetworkPublic("prepare-initial-node-record", preparationJSON(t, recordPlan))
	if err != nil || name != "node-record.signing-input" || len(unsigned) == 0 {
		t.Fatalf("portable Record preparation failed: %q: %v", name, err)
	}
	epochPlan, epochKey, _, signedEpoch, _ := publicEpochFixture(t)
	defer clear(epochKey)
	name, unsigned, err = PrepareNetworkPublic("prepare-initial-epoch", preparationJSON(t, epochPlan))
	if err != nil || name != "epoch.unsigned" || len(unsigned) >= len(signedEpoch.Raw) ||
		!bytes.Equal(unsigned, signedEpoch.Raw[:len(unsigned)]) {
		t.Fatalf("portable Epoch does not match the signed fixture prefix: %q: %v", name, err)
	}
	profilePlan, profileKey, body, _ := signingProfileFixture(t, time.Unix(1800003600, 0).UTC().Add(time.Minute))
	defer clear(profileKey)
	name, unsigned, err = PrepareNetworkPublic("prepare-closed-profile", preparationJSON(t, profilePlan))
	if err != nil || name != "profile.unsigned" || !bytes.Equal(unsigned, body) {
		t.Fatalf("portable profile differs from the independent body: %q: %v", name, err)
	}
}
