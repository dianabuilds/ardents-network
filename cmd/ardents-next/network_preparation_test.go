package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func preparationJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func preparationHex(raw []byte) string    { return hex.EncodeToString(raw) }
func preparationIdentifier(n byte) string { v := [32]byte{n}; return preparationHex(v[:]) }

func preparationRecordPlan(public ed25519.PublicKey) map[string]any {
	return map[string]any{"schema": "ardents-initial-node-record-plan-v1", "network_id": preparationIdentifier(1), "node_id": preparationIdentifier(2), "valid_from": "2033-05-22T05:20:00Z", "valid_until": "2033-05-22T06:20:00Z", "family": "family-one", "endpoint": "127.0.0.1:4401", "carrier": "ardents-carrier-tcp-tls-v2", "capacity": 3, "public_key": preparationHex(public)}
}

func TestNetworkPreparationRecordIndependentBytesAndStrictInputs(t *testing.T) {
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32))
	public := key.Public().(ed25519.PublicKey)
	p := preparationRecordPlan(public)
	name, got, err := prepareNetworkPublic("prepare-initial-node-record", preparationJSON(t, p))
	if err != nil || name != "node-record.signing-input" {
		t.Fatal(name, err)
	}
	from, _ := time.Parse(time.RFC3339, p["valid_from"].(string))
	until, _ := time.Parse(time.RFC3339, p["valid_until"].(string))
	f, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: [32]byte{1}, NodeID: [32]byte{2}, Generation: 1, ValidFrom: from, ValidUntil: until, Family: "family-one", Endpoint: "127.0.0.1:4401", Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 3, PrivateKey: key})
	if err != nil || !bytes.Equal(got, f.Raw[:len(f.Raw)-64]) {
		t.Fatal("independent record bytes", err)
	}
	for _, change := range []struct {
		field string
		value any
	}{
		{"network_id", []byte{1}}, {"network_id", make([]int, 33)}, {"node_id", "00"}, {"public_key", strings.Repeat("a", 66)},
		{"public_key", strings.ToUpper(preparationHex(public))}, {"capacity", 1.5}, {"capacity", 0}, {"capacity", 1025},
		{"valid_from", "2033-05-22T05:20:00.1Z"}, {"valid_until", "2033-05-22T06:20:00+00:00"},
		{"family", "has space"}, {"endpoint", "bad"}, {"carrier", "ardents-carrier-quic-v2"}, {"carrier", "ardents-carrier-tcp-tls-v1"}, {"schema", "ardents-qualification-node-record-plan-v1"},
		{"private_key", "forbidden"}, {"epoch", 1}, {"Network_ID", preparationIdentifier(1)},
	} {
		t.Run(change.field+"-"+string(preparationJSON(t, change.value)), func(t *testing.T) {
			q := preparationRecordPlan(public)
			q[change.field] = change.value
			if _, b, e := prepareNetworkPublic("prepare-initial-node-record", preparationJSON(t, q)); e == nil || b != nil {
				t.Fatal("invalid public plan accepted")
			}
		})
	}
	raw := preparationJSON(t, p)
	for _, bad := range [][]byte{append(bytes.Clone(raw), raw...), append(bytes.Clone(raw), []byte(" true")...), []byte("[]"), []byte("null"), bytes.Replace(raw, []byte(`"capacity":3`), []byte(`"capacity":3,"capacity":3`), 1), bytes.Replace(raw, []byte(`"capacity":3`), []byte(`"capacity":null`), 1)} {
		if _, b, e := prepareNetworkPublic("prepare-initial-node-record", bad); e == nil || b != nil {
			t.Fatal("duplicate/trailing/null plan accepted", string(bad))
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if code := runNetworkPreparation(ctx, []string{"prepare-initial-node-record", filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "fresh")}, io.Discard); code != 130 {
		t.Fatal(code)
	}
}
