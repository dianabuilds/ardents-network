//go:build linux

package endpoint

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func participantStockFacts(t *testing.T) (admission.AuthorityFacts, time.Time) {
	t.Helper()
	type algorithm struct {
		OID    asn1.ObjectIdentifier
		Params asn1.RawValue
	}
	type parameters struct {
		Hash algorithm `asn1:"explicit,tag:0"`
		MGF  algorithm `asn1:"explicit,tag:1"`
		Salt int       `asn1:"explicit,tag:2"`
	}
	type subject struct {
		Algorithm algorithm
		Bits      asn1.BitString
	}
	hash := algorithm{asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}, asn1.RawValue{FullBytes: []byte{5, 0}}}
	hashBytes, err := asn1.Marshal(hash)
	if err != nil {
		t.Fatal(err)
	}
	params, err := asn1.Marshal(parameters{hash, algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}, asn1.RawValue{FullBytes: hashBytes}}, 48})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1800000000, 0).UTC()
	window := now.Truncate(time.Hour)
	p := admission.AuthorityFacts{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, Digest: [32]byte{4}, IssuanceAuthorityKey: [32]byte{5}, IssuerNodeID: [32]byte{6}, IssuerDutyGeneration: 1, NotBefore: window, NotAfter: window.Add(time.Hour), TokenKeyCount: 3}
	for i := 0; i < 3; i++ {
		n := new(big.Int).Lsh(big.NewInt(1), 2047)
		n.Add(n, big.NewInt(int64(2*i+1)))
		raw := x509.MarshalPKCS1PublicKey(&rsa.PublicKey{N: n, E: 65537})
		encoded, err := asn1.Marshal(subject{algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}, asn1.RawValue{FullBytes: params}}, asn1.BitString{Bytes: raw, BitLength: len(raw) * 8}})
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) != len(p.TokenKeys[i].SPKI) {
			t.Fatalf("unexpected key size %d", len(encoded))
		}
		p.TokenKeys[i].WindowStart, p.TokenKeys[i].Class = window, uint8(i+1)
		copy(p.TokenKeys[i].SPKI[:], encoded)
	}
	if err := p.ValidateAt(now); err != nil {
		t.Fatal(err)
	}
	return p, now
}

func TestParticipantStockRequiresExistingHistory(t *testing.T) {
	facts, now := participantStockFacts(t)
	observe := func() (admission.AuthorityFacts, time.Time, error) { return facts, now, nil }
	check := func() error { return nil }
	root := filepath.Join(t.TempDir(), "reader")
	owner, err := openParticipantStock(root, admission.AllocationUser, observe, check)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err = openParticipantStock(root, admission.AllocationUser, observe, check)
	if err != nil {
		t.Fatalf("retained history refused: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	owner, err = openParticipantStock(root, admission.AllocationUser, observe, check)
	if err == nil {
		owner.Close()
		t.Fatal("erased existing history accepted")
	}
	entries, err = os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("refusal modified history: %v, %v", entries, err)
	}
}
