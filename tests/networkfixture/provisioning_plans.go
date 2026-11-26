package networkfixture

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/closedprofile"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// These plans and expected bytes are fixtures for the separate preparation
// and signing owners. Their seeds and signed vectors are local to tests.
func PlanJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func PlanIdentifier(n byte) string { v := [32]byte{n}; return hex.EncodeToString(v[:]) }

func RecordPlan(public ed25519.PublicKey) map[string]any {
	return map[string]any{"schema": "ardents-initial-node-record-plan-v1", "network_id": PlanIdentifier(1), "node_id": PlanIdentifier(2), "valid_from": "2033-05-22T05:20:00Z", "valid_until": "2033-05-22T06:20:00Z", "family": "family-one", "endpoint": "127.0.0.1:4401", "carrier": "ardents-carrier-tcp-tls-v2", "capacity": 3, "public_key": hex.EncodeToString(public)}
}

func EpochPlan(t *testing.T) (map[string]any, ed25519.PrivateKey, [][]byte, networkfixture.Epoch, time.Time) {
	t.Helper()
	from, _ := time.Parse(time.RFC3339, "2033-05-22T05:20:00Z")
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
	p := map[string]any{"schema": "ardents-initial-epoch-plan-v1", "network_id": PlanIdentifier(1), "assignment_seed": PlanIdentifier(13), "valid_from": from.Format(time.RFC3339), "valid_until": until.Format(time.RFC3339), "domains": domains, "records": []string{hex.EncodeToString(inputs[0]), hex.EncodeToString(inputs[1])}, "authority_keys": []string{hex.EncodeToString(stateKey.Public().(ed25519.PublicKey))}}
	return p, stateKey, inputs, f, reference
}
func ProfilePlan(t *testing.T, now time.Time) (map[string]any, ed25519.PrivateKey, []byte, []byte) {
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
		nodes = append(nodes, map[string]any{"node_id": hex.EncodeToString(n.NodeID[:]), "record_digest": hex.EncodeToString(n.RecordDigest[:]), "role_domain": n.RoleDomain, "subrole": n.Subrole, "duty_generation": n.DutyGeneration})
	}
	for _, k := range p.Keys {
		keys = append(keys, map[string]any{"window_start": time.Unix(int64(k.WindowStart), 0).UTC().Format(time.RFC3339), "class": k.Class, "spki": hex.EncodeToString(k.SPKI)})
	}
	plan := map[string]any{"schema": "ardents-closed-profile-plan-v1", "network_id": PlanIdentifier(1), "state_generation": hex.EncodeToString(f.Epoch.Digest[:]), "epoch_digest": hex.EncodeToString(f.Epoch.Digest[:]), "issuer_node_id": PlanIdentifier(2), "issuance_authority_key": hex.EncodeToString(permission[:]), "epoch": 1, "not_before": p.NotBefore.Format(time.RFC3339), "not_after": p.NotAfter.Format(time.RFC3339), "nodes": nodes, "token_keys": keys}
	return plan, state, bytes.Clone(f.Profile[:len(f.Profile)-64]), f.Profile
}

func RecordSigningPlan(t *testing.T) (map[string]any, ed25519.PrivateKey, []byte, time.Time) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	p := RecordPlan(key.Public().(ed25519.PublicKey))
	from := time.Date(2033, 5, 22, 5, 20, 0, 0, time.UTC)
	record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: [32]byte{1}, NodeID: [32]byte{2}, Generation: 1, ValidFrom: from, ValidUntil: from.Add(time.Hour), Family: "family-one", Endpoint: "127.0.0.1:4401", Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 3, PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	reference, _ := time.Parse(time.RFC3339, "2033-05-22T05:30:00Z")
	return p, key, record.Raw[:len(record.Raw)-ed25519.SignatureSize], reference
}

func SigningDeclaration(t *testing.T, purpose string, plan any, reference time.Time) []byte {
	t.Helper()
	field := purpose
	if purpose == "node-record" {
		field = "record"
	}
	return PlanJSON(t, map[string]any{"schema": "ardents-initial-" + purpose + "-signing-plan-v1", "reference_time": reference.Format(time.RFC3339), field: plan})
}

func ProfileSigningDeclaration(t *testing.T, plan any, public ed25519.PublicKey, reference time.Time) []byte {
	t.Helper()
	return PlanJSON(t, map[string]any{"schema": "ardents-closed-profile-signing-plan-v1", "reference_time": reference.Format(time.RFC3339), "state_authority_key": hex.EncodeToString(public), "profile": plan})
}

func CertificateDeclaration(t *testing.T, record any, reference time.Time) []byte {
	t.Helper()
	return PlanJSON(t, map[string]any{"schema": "ardents-node-certificate-plan-v1", "reference_time": reference.Format(time.RFC3339Nano), "record": record})
}
