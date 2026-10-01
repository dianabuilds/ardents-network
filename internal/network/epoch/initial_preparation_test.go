package epoch_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestInitialClosedPreparationMatchesIndependentEncodingAndOrdinaryVerification(t *testing.T) {
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public := authority.Public().(ed25519.PublicKey)
	now := time.Unix(2_000_400_000, 0).UTC()
	network := [32]byte{9}
	input := epoch.InitialClosedEpoch{NetworkID: network, AssignmentSeed: [32]byte{8}, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Domains: []string{"a", "b"}, AuthorityKeys: []ed25519.PublicKey{public}}
	var fixtureRecords []networkfixture.Record
	for index := 0; index < 3; index++ {
		_, nodeKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		carrier := epoch.CarrierClosedTCP
		if index == 1 {
			carrier = epoch.CarrierClosedQUIC
		}
		record := epoch.InitialClosedRecord{NetworkID: network, NodeID: [32]byte{byte(3 - index)}, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(2 * time.Hour), Family: []string{"family-one", "family-two", "family-one"}[index], Endpoint: []string{"127.0.0.1:4401", "127.0.0.1:4402", "127.0.0.1:4403"}[index], Carrier: carrier, Capacity: uint16(index + 1), PublicKey: nodeKey.Public().(ed25519.PublicKey)}
		message, err := epoch.PrepareInitialClosedRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		raw := append(append([]byte(nil), message...), ed25519.Sign(nodeKey, message)...)
		fixture, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: network, NodeID: record.NodeID, Generation: 1, ValidFrom: record.ValidFrom, ValidUntil: record.ValidUntil, Family: record.Family, Endpoint: record.Endpoint, Carrier: carrier, Capability: 2, Capacity: record.Capacity, PrivateKey: nodeKey})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, fixture.Raw) {
			t.Fatal("maintained initial record differs from independent grammar")
		}
		input.Records = append(input.Records, raw)
		fixtureRecords = append(fixtureRecords, fixture)
	}
	message, err := epoch.PrepareInitialClosedEpoch(input)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	id := sha256.Sum256(public)
	raw := append(append([]byte(nil), message...), 1)
	raw = append(raw, id[:]...)
	raw = append(raw, ed25519.Sign(authority, digest[:])...)
	fixture, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{NetworkID: network, Number: 1, ValidFrom: input.ValidFrom, ValidUntil: input.ValidUntil, Inputs: input.Records, Accepted: fixtureRecords, AssignmentSeed: input.AssignmentSeed, Profile: epoch.ProfileClosedRoute, Version: 3, Domains: input.Domains, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, fixture.Raw) {
		t.Fatal("maintained initial Epoch differs from independent commitment/assignment encoding")
	}
	policy := epoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{id: public}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: now}
	decision, err := epoch.Verify(policy, raw, input.Records, fixture.Materials, true)
	if err != nil || len(decision.Candidates) != 3 {
		t.Fatalf("ordinary Epoch verification: %v", err)
	}
	for index, want := range fixture.Materials {
		got, err := decision.Materialization(uint32(index))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("materialization mismatch: %v", err)
		}
	}
	for name, mutate := range map[string]func(*epoch.InitialClosedEpoch){
		"forged node signature": func(v *epoch.InitialClosedEpoch) {
			v.Records = append([][]byte(nil), v.Records...)
			v.Records[0] = append([]byte(nil), v.Records[0]...)
			v.Records[0][len(v.Records[0])-1] ^= 1
		},
		"duplicate node": func(v *epoch.InitialClosedEpoch) {
			v.Records = append(append([][]byte(nil), v.Records...), v.Records[0])
		},
		"wrong Network": func(v *epoch.InitialClosedEpoch) { v.NetworkID[0] ^= 1 },
		"authority Node key collision": func(v *epoch.InitialClosedEpoch) {
			v.AuthorityKeys = append(append([]ed25519.PublicKey(nil), v.AuthorityKeys...), ed25519.PublicKey(v.Records[0][len(v.Records[0])-96:len(v.Records[0])-64]))
		},
		"unsorted domains":       func(v *epoch.InitialClosedEpoch) { v.Domains = []string{"b", "a"} },
		"Epoch outlives records": func(v *epoch.InitialClosedEpoch) { v.ValidUntil = now.Add(3 * time.Hour) },
		"fractional validity":    func(v *epoch.InitialClosedEpoch) { v.ValidFrom = v.ValidFrom.Add(time.Nanosecond) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := input
			mutate(&changed)
			if value, err := epoch.PrepareInitialClosedEpoch(changed); err == nil || value != nil {
				t.Fatal("invalid initial closed Epoch produced signing input")
			}
		})
	}
	forged := append([]byte(nil), raw...)
	forged[len(forged)-1] ^= 1
	if _, err := epoch.Verify(policy, forged, input.Records, fixture.Materials, true); err == nil {
		t.Fatal("ordinary owner accepted forged State signature")
	}
}

func TestInitialClosedRecordPreparationRefusesInvalidFacts(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	input := epoch.InitialClosedRecord{NetworkID: [32]byte{1}, NodeID: [32]byte{2}, ValidFrom: now, ValidUntil: now.Add(time.Hour), Family: "operator-family", Endpoint: "127.0.0.1:4401", Carrier: epoch.CarrierClosedTCP, Capacity: 1, PublicKey: public}
	for name, mutate := range map[string]func(*epoch.InitialClosedRecord){
		"missing Network":    func(v *epoch.InitialClosedRecord) { v.NetworkID = [32]byte{} },
		"missing Node":       func(v *epoch.InitialClosedRecord) { v.NodeID = [32]byte{} },
		"bad key":            func(v *epoch.InitialClosedRecord) { v.PublicKey = []byte{1} },
		"legacy Carrier":     func(v *epoch.InitialClosedRecord) { v.Carrier = epoch.CarrierLegacyTCP },
		"family whitespace":  func(v *epoch.InitialClosedRecord) { v.Family = "family name" },
		"invalid endpoint":   func(v *epoch.InitialClosedRecord) { v.Endpoint = "not-an-endpoint" },
		"zero port":          func(v *epoch.InitialClosedRecord) { v.Endpoint = "127.0.0.1:0" },
		"negative timestamp": func(v *epoch.InitialClosedRecord) { v.ValidFrom = time.Unix(-1, 0).UTC() },
		"capacity":           func(v *epoch.InitialClosedRecord) { v.Capacity = 1025 },
	} {
		t.Run(name, func(t *testing.T) {
			changed := input
			mutate(&changed)
			if message, err := epoch.PrepareInitialClosedRecord(changed); err == nil || message != nil {
				t.Fatal("invalid Node facts produced signing input")
			}
		})
	}
}
