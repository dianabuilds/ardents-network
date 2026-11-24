//go:build linux && (network_preparation_native || network_signing_native)

package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"
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
