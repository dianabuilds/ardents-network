package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestForwardingRecordPreparationKeepsIdentityKeyAndPurposeSeparate(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	args := []string{"--network-id", strings.Repeat("01", 32), "--node-id", strings.Repeat("02", 32), "--node-key", strings.Repeat("03", 32), "--family", "declared-family", "--endpoint", "127.0.0.1:9401", "--valid-from", now.Add(-10 * time.Minute).Format(time.RFC3339), "--valid-until", now.Add(time.Hour).Format(time.RFC3339), "--capacity", "1", "--output", "/var/lib/provision/record"}
	p, path, err := forwardingRecordArguments(args, now)
	if err != nil || path != "/var/lib/provision/record" || p.Node == p.PublicKey {
		t.Fatal("public identity declaration refused or collapsed", err)
	}
	prepared, err := p.prepare(now)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	_, reopened, err := decodeForwardingRecordPlan(raw, now)
	if err != nil || !bytes.Equal(prepared, reopened) {
		t.Fatal("canonical public declaration changed", err)
	}
	for _, change := range []struct {
		index int
		value string
	}{{1, strings.Repeat("00", 32)}, {3, "bad"}, {5, strings.Repeat("AA", 32)}, {9, "example.invalid:9401"}, {9, "127.0.0.1:443"}, {9, "127.0.0.1:09401"}, {13, now.Add(8 * 24 * time.Hour).Format(time.RFC3339)}, {15, "0"}, {15, "1025"}, {17, "relative"}} {
		candidate := append([]string(nil), args...)
		candidate[change.index] = change.value
		if _, _, err := forwardingRecordArguments(candidate, now); err == nil {
			t.Fatal("invalid/off-purpose declaration accepted", change)
		}
	}
	for _, candidate := range [][]string{args[:16], append(append([]string(nil), args...), "--carrier", "quic"), append(append([]string(nil), args...), "--node-id", p.Node)} {
		if _, _, err := forwardingRecordArguments(candidate, now); err == nil {
			t.Fatal("missing, duplicate or deferred Carrier declaration accepted")
		}
	}
	for _, clock := range []time.Time{now.Add(6 * time.Minute), now.Add(-2 * time.Minute)} {
		if _, _, err := decodeForwardingRecordPlan(raw, clock); err == nil {
			t.Fatal("stale/future frozen reference accepted", clock)
		}
	}
	for _, candidate := range [][]byte{raw[:len(raw)-1], append(bytes.Clone(raw), '\n'), bytes.Replace(raw, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1), bytes.Replace(raw, []byte("ardents-forwarding-node-record-plan-v1"), []byte("ardents-source-initial-network-plan-v1"), 1)} {
		if _, _, err := decodeForwardingRecordPlan(candidate, now); err == nil {
			t.Fatal("noncanonical or wrong-purpose plan accepted")
		}
	}
}
