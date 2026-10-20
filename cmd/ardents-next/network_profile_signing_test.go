//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

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

func TestNetworkSigningIndependentProfileAndAuthority(t *testing.T) {
	now := time.Unix(1800003600, 0).UTC().Add(time.Minute)
	p, key, body, want := signingProfileFixture(t, now)
	defer clear(key)
	raw := profileSigningDeclaration(t, p, key.Public().(ed25519.PublicKey), now)
	digest := sha256.Sum256(body)
	pin := preparationHex(digest[:])
	admit := func(raw, body []byte, pin string) *networkSigning {
		t.Helper()
		s, e := admitNetworkSigning("sign-closed-profile", raw, body, pin)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s := admit(raw, body, pin)
	if _, e := admitNetworkSigning("sign-closed-profile", profileSigningDeclaration(t, p, ed25519.PublicKey(bytes.Repeat([]byte{0}, 32)), now), body, pin); e == nil {
		t.Fatal("zero State pin accepted")
	}
	permission := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{83}, 32))
	if _, e := admitNetworkSigning("sign-closed-profile", profileSigningDeclaration(t, p, permission.Public().(ed25519.PublicKey), now), body, pin); e == nil {
		t.Fatal("Admission authority aliased as State signer")
	}
	clear(permission)
	outputs, err := s.sign(key, now)
	if err != nil || len(outputs) != 1 || outputs[0].name != "profile.bin" || !bytes.Equal(outputs[0].raw, want) {
		t.Fatal("independent profile vector differs", err)
	}
	for _, fault := range []string{"prepared", "pin", "duplicate", "unknown", "mixed", "trailing", "wrong purpose", "missing authority", "zero authority", "authority shape", "fraction", "foreign context", "bad spki", "duplicate node", "long interval"} {
		t.Run(fault, func(t *testing.T) {
			r, b, pin := bytes.Clone(raw), bytes.Clone(body), pin
			var q map[string]any
			_ = json.Unmarshal(r, &q)
			switch fault {
			case "prepared":
				b[0] ^= 1
			case "pin":
				pin = preparationIdentifier(99)
			case "duplicate":
				r = bytes.Replace(r, []byte(`"schema":`), []byte(`"schema":"x","schema":`), 1)
			case "unknown":
				q["private_key"] = true
				r = preparationJSON(t, q)
			case "mixed":
				q["epoch"] = p
				r = preparationJSON(t, q)
			case "trailing":
				r = append(r, []byte(" null")...)
			case "wrong purpose":
				q["schema"] = "ardents-initial-epoch-signing-plan-v1"
				r = preparationJSON(t, q)
			case "missing authority":
				delete(q, "state_authority_key")
				r = preparationJSON(t, q)
			case "zero authority":
				q["state_authority_key"] = preparationIdentifier(0)
				r = preparationJSON(t, q)
			case "authority shape":
				q["state_authority_key"] = []int{1}
				r = preparationJSON(t, q)
			case "fraction":
				q["reference_time"] = now.Add(time.Nanosecond).Format(time.RFC3339Nano)
				r = preparationJSON(t, q)
			default:
				profile := q["profile"].(map[string]any)
				switch fault {
				case "foreign context":
					profile["network_id"] = preparationIdentifier(99)
				case "bad spki":
					profile["token_keys"].([]any)[0].(map[string]any)["spki"] = "00"
				case "duplicate node":
					n := profile["nodes"].([]any)[0]
					profile["nodes"] = []any{n, n}
				case "long interval":
					profile["not_after"] = now.Add(7 * time.Hour).Format(time.RFC3339)
				}
				r = preparationJSON(t, q)
			}
			if _, e := admitNetworkSigning("sign-closed-profile", r, b, pin); e == nil {
				t.Fatal("invalid profile binding accepted")
			}
			if _, e := admit(raw, body, preparationHex(digest[:])).sign(key, now); e != nil {
				t.Fatal("healthy control", e)
			}
		})
	}
	for _, n := range []byte{82, 83, 84} {
		foreign := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{n}, 32))
		if _, e := s.sign(foreign, now); e == nil {
			t.Fatal("Node/Admission/foreign private key accepted")
		}
		if n != 83 {
			wrong := admit(profileSigningDeclaration(t, p, foreign.Public().(ed25519.PublicKey), now), body, pin)
			if _, e := wrong.sign(key, now); e == nil {
				t.Fatal("independent State pin ignored")
			}
		}
		clear(foreign)
	}
	for _, at := range []time.Time{now.Add(-time.Minute - time.Nanosecond), now.Add(5*time.Minute + time.Nanosecond), s.until} {
		if _, e := s.sign(key, at); e == nil {
			t.Fatal("expired original time accepted")
		}
	}
}
