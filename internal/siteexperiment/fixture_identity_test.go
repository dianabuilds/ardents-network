package siteexperiment

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"
)

func TestFixtureBindsNameTargetRunNetworkNonceAndCredential(t *testing.T) {
	t.Parallel()
	now := time.Unix(1786400000, 0).UTC()
	fixture, err := newAuthorityFixture("gatec-run-1", "gatec-network-1", now, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nonce := bytes.Repeat([]byte{0x42}, 32)
	nameRecord, err := fixture.signedNameRecord(nonce, now.Add(15*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	name, err := verifyNameRecord(nameRecord, fixture.namePublic, "gatec-run-1", "gatec-network-1", nonce, now)
	if err != nil {
		t.Fatal(err)
	}
	if name.Name != "site.reference" || name.Target != fixture.target || name.NameGeneration != 1 || name.NameRevision != 1 {
		t.Fatalf("name record = %#v", name)
	}
	descriptorRecord, err := fixture.signedDescriptor(nonce, now.Add(15*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := verifyDescriptor(descriptorRecord, fixture.servicePublic, "gatec-run-1", "gatec-network-1", nonce, fixture.target, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.InstanceGeneration != 1 || descriptor.Endpoint == "" {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}

func TestFixtureRejectsWrongSignatureContextFreshnessAndBinding(t *testing.T) {
	t.Parallel()
	now := time.Unix(1786400000, 0).UTC()
	fixture, err := newAuthorityFixture("gatec-run-1", "gatec-network-1", now, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nonce := bytes.Repeat([]byte{0x24}, 32)
	record, err := fixture.signedNameRecord(nonce, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		data    []byte
		run     string
		network string
		nonce   []byte
		now     time.Time
	}{
		{name: "modified", data: append(bytes.Clone(record), 'x'), run: "gatec-run-1", network: "gatec-network-1", nonce: nonce, now: now},
		{name: "wrong run", data: record, run: "gatec-run-2", network: "gatec-network-1", nonce: nonce, now: now},
		{name: "wrong network", data: record, run: "gatec-run-1", network: "gatec-network-2", nonce: nonce, now: now},
		{name: "nonce mismatch", data: record, run: "gatec-run-1", network: "gatec-network-1", nonce: bytes.Repeat([]byte{0x25}, 32), now: now},
		{name: "stale", data: record, run: "gatec-run-1", network: "gatec-network-1", nonce: nonce, now: now.Add(2 * time.Second)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := verifyNameRecord(test.data, fixture.namePublic, test.run, test.network, test.nonce, test.now); err == nil {
				t.Fatal("invalid Name Record accepted")
			}
		})
	}
}

func TestFixtureMigrationSupersedesOldInstanceWithoutChangingNameOrTarget(t *testing.T) {
	t.Parallel()
	now := time.Unix(1786400000, 0).UTC()
	fixture, err := newAuthorityFixture("gatec-run-1", "gatec-network-1", now, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nonce := bytes.Repeat([]byte{0x33}, 32)
	oldRecord, err := fixture.signedDescriptor(nonce, now.Add(15*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	oldTarget := fixture.target
	oldPublic := hex.EncodeToString(fixture.instancePublic)
	if err := fixture.migrate(now.Add(time.Second), rand.Reader); err != nil {
		t.Fatal(err)
	}
	if fixture.target != oldTarget || hex.EncodeToString(fixture.instancePublic) == oldPublic || fixture.instanceGeneration != 2 {
		t.Fatal("migration did not preserve Target while replacing Instance")
	}
	if _, err := verifyDescriptor(oldRecord, fixture.servicePublic, "gatec-run-1", "gatec-network-1", nonce, fixture.target, 2, now.Add(time.Second)); err == nil {
		t.Fatal("superseded generation accepted")
	}
	newRecord, err := fixture.signedDescriptor(nonce, now.Add(16*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyDescriptor(newRecord, fixture.servicePublic, "gatec-run-1", "gatec-network-1", nonce, fixture.target, 2, now.Add(time.Second)); err != nil {
		t.Fatalf("generation 2 rejected: %v", err)
	}
}
