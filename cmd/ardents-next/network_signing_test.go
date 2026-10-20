//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func signingRecordFixture(t *testing.T) (map[string]any, ed25519.PrivateKey, []byte, time.Time) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, 32))
	p := preparationRecordPlan(key.Public().(ed25519.PublicKey))
	_, message, err := prepareNetworkPublic("prepare-initial-node-record", preparationJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	reference, _ := admissionTime("2033-05-22T05:30:00Z")
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

func TestNetworkSigningIndependentRecordAndBinding(t *testing.T) {
	p, key, message, reference := signingRecordFixture(t)
	digest := sha256.Sum256(message)
	raw := signingDeclaration(t, "node-record", p, reference)
	admit := func(raw, prepared []byte, pin string) *initialNetworkSigning {
		t.Helper()
		s, err := admitInitialNetworkSigning("sign-initial-node-record", raw, prepared, pin)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := admit(raw, message, preparationHex(digest[:]))
	from, _ := admissionTime(p["valid_from"].(string))
	until, _ := admissionTime(p["valid_until"].(string))
	f, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: [32]byte{1}, NodeID: [32]byte{2}, Generation: 1, ValidFrom: from, ValidUntil: until, Family: "family-one", Endpoint: "127.0.0.1:4401", Carrier: p["carrier"].(string), Capability: 2, Capacity: 3, PrivateKey: key})
	outputs, signErr := s.sign(key, reference)
	if err != nil || signErr != nil || len(outputs) != 1 || outputs[0].name != "node-record.bin" || !bytes.Equal(outputs[0].raw, f.Raw) {
		t.Fatal("independent signature transcript mismatch", err, signErr)
	}
	for _, fault := range []string{"prepared", "pin", "mixed", "duplicate", "trailing", "wrong-purpose", "fraction", "unknown", "quic"} {
		t.Run(fault, func(t *testing.T) {
			r, prepared, pin := bytes.Clone(raw), bytes.Clone(message), preparationHex(digest[:])
			switch fault {
			case "prepared":
				prepared[0] ^= 1
			case "pin":
				pin = preparationIdentifier(99)
			case "mixed":
				r = bytes.Replace(r, []byte(`"record":`), []byte(`"epoch":{},"record":`), 1)
			case "duplicate":
				r = bytes.Replace(r, []byte(`"schema":`), []byte(`"schema":"duplicate","schema":`), 1)
			case "trailing":
				r = append(r, []byte(" null")...)
			case "wrong-purpose":
				r = signingDeclaration(t, "epoch", p, reference)
			case "fraction":
				r = bytes.Replace(r, []byte("05:30:00Z"), []byte("05:30:00.1Z"), 1)
			case "unknown":
				r = bytes.Replace(r, []byte(`"record":`), []byte(`"private":true,"record":`), 1)
			case "quic":
				r = bytes.ReplaceAll(r, []byte("ardents-carrier-tcp-tls-v2"), []byte("ardents-carrier-quic-v2"))
			}
			if _, err := admitInitialNetworkSigning("sign-initial-node-record", r, prepared, pin); err == nil {
				t.Fatal("invalid binding accepted")
			}
			if _, err := admit(raw, message, preparationHex(digest[:])).sign(key, reference); err != nil {
				t.Fatal("healthy control", err)
			}
		})
	}
	wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, 32))
	if _, err = s.sign(wrong, reference); err == nil {
		t.Fatal("foreign key signed")
	}
	for _, now := range []time.Time{reference.Add(-time.Minute - time.Nanosecond), reference.Add(5*time.Minute + time.Nanosecond), from.Add(-time.Nanosecond), until} {
		if _, err = s.sign(key, now); err == nil {
			t.Fatal("invalid actual time signed", now)
		}
	}
	for _, change := range []struct{ from, until, ref time.Time }{
		{from, until, from.Add(-time.Second)}, {from, until, until}, {from, from.Add(7*24*time.Hour + time.Second), reference},
	} {
		q := *s
		q.from, q.until, q.reference = change.from, change.until, change.ref
		if q.checkTime(reference) == nil {
			t.Fatal("invalid interval accepted")
		}
	}
}

func TestNetworkSigningPrivateEncoding(t *testing.T) {
	_, key, _, _ := signingRecordFixture(t)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(der)
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	defer clear(raw)
	parsed, err := parseInitialNetworkKey(raw)
	if err != nil || !bytes.Equal(parsed, key) {
		t.Fatal("valid key", err)
	}
	clear(parsed)
	for _, bad := range [][]byte{nil, append(bytes.Clone(raw), ' '), append([]byte("prefix\n"), raw...), append(bytes.Clone(raw), raw...), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2}}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: append(bytes.Clone(der), 0)})} {
		got, err := parseInitialNetworkKey(bad)
		clear(got)
		clear(bad)
		if err == nil {
			t.Fatal("malformed key accepted")
		}
	}
}

func TestNetworkSigningIndependentEpochOrdering(t *testing.T) {
	from, _ := admissionTime("2033-05-22T05:20:00Z")
	until := from.Add(time.Hour)
	reference := from.Add(time.Minute)
	stateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, 32))
	defer clear(stateKey)
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
	_, message, err := prepareNetworkPublic("prepare-initial-epoch", preparationJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	s, err := admitInitialNetworkSigning("sign-initial-epoch", signingDeclaration(t, "epoch", p, reference), message, preparationHex(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := s.sign(stateKey, reference)
	if err != nil || len(outputs) != 5 || outputs[0].name != "epoch.bin" || !bytes.Equal(outputs[0].raw, f.Raw) {
		t.Fatal("independent Epoch transcript", err)
	}
	for i := range inputs {
		if outputs[1+i].name != fmt.Sprintf("input-%02d.bin", i) || !bytes.Equal(outputs[1+i].raw, inputs[i]) || outputs[3+i].name != fmt.Sprintf("material-%02d.bin", i) || !bytes.Equal(outputs[3+i].raw, f.Materials[i]) {
			t.Fatal("Source input/accepted View order differs", i)
		}
	}
}
