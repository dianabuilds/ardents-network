package namespace_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

func TestEd25519RFC8032Vector(t *testing.T) {
	t.Parallel()
	public, _ := hex.DecodeString("d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a")
	signature, _ := hex.DecodeString("e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e06522490155" +
		"5fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b")
	if !ed25519.Verify(ed25519.PublicKey(public), nil, signature) {
		t.Fatal("Go Ed25519 failed RFC 8032 test vector 1")
	}
	seed, _ := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
	if got := ed25519.Sign(ed25519.NewKeyFromSeed(seed), nil); !bytes.Equal(got, signature) {
		t.Fatal("Go Ed25519 signing differs from RFC 8032 test vector 1")
	}
}

func TestSignedRecordBindsAuthorityNetworkAndCanonicalRecord(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	network := [32]byte{1, 2, 3}
	value := record.Record{Name: "alice", Generation: 1, Revision: 4,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(public), Target: [32]byte{1},
		LeaseExpiresAt: 200, GraceExpiresAt: 220, RecordNotAfter: 190_000}

	signed, err := record.SignRecord(network, value, private)
	if err != nil {
		t.Fatalf("SignRecord: %v", err)
	}
	opened, err := record.VerifyRecord(network, signed)
	if err != nil {
		t.Fatalf("VerifyRecord: %v", err)
	}
	if opened != value {
		t.Fatalf("opened record = %+v, want %+v", opened, value)
	}

	wrongNetwork := network
	wrongNetwork[0] ^= 0xff
	if _, err := record.VerifyRecord(wrongNetwork, signed); err == nil {
		t.Fatal("signature was replayed across networks")
	}
	for _, mutate := range []func([]byte) []byte{
		func(raw []byte) []byte { raw[len(raw)-1] ^= 1; return raw },
		func(raw []byte) []byte { raw[12] ^= 1; return raw },
		func(raw []byte) []byte { return append(raw, 0) },
	} {
		changed := append([]byte(nil), signed...)
		changed = mutate(changed)
		if _, err := record.VerifyRecord(network, changed); err == nil {
			t.Fatal("modified signed record was accepted")
		}
	}
}

func TestSignedRecordBindsEveryLifecycleField(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	network := [32]byte{9, 8, 7}
	base := record.Record{Name: "leaf.sub.root", Generation: 3, Revision: 4,
		Lease: "active", Consistency: "fork", Recovery: "recovery-pending",
		Authority: hex.EncodeToString(public), Target: [32]byte{1}, ParentName: "sub.root",
		ParentGeneration: 2, LeaseExpiresAt: 200, GraceExpiresAt: 220,
		RecordNotAfter:    190_000,
		RecoveryStartedAt: 1_000, RecoveryExpiresAt: 1_000 + (72 * time.Hour).Milliseconds(),
		RecoveryOperation: [32]byte{3}, RecoverySuccessor: [32]byte{4},
		RecoveryPolicy: [32]byte{5}, RecoveryPolicyRev: 1,
		RecoveryPolicyDelay: (72 * time.Hour).Milliseconds(), Continuity: 6, ConflictIdentifier: "fork-a"}
	signed, err := record.SignRecord(network, base, private)
	if err != nil {
		t.Fatal(err)
	}

	mutations := map[string]func(*record.Record){
		"name":        func(r *record.Record) { r.Name = "other.sub.root" },
		"generation":  func(r *record.Record) { r.Generation++ },
		"revision":    func(r *record.Record) { r.Revision++ },
		"lease":       func(r *record.Record) { r.Lease = "grace" },
		"consistency": func(r *record.Record) { r.Consistency = "unavailable" },
		"recovery": func(r *record.Record) {
			r.Recovery = "stable"
			r.RecoveryStartedAt, r.RecoveryExpiresAt = 0, 0
			r.RecoveryOperation, r.RecoverySuccessor = [32]byte{}, [32]byte{}
		},
		"authority": func(r *record.Record) {
			r.Authority = hex.EncodeToString(bytes.Repeat([]byte{7}, ed25519.PublicKeySize))
		},
		"target":            func(r *record.Record) { r.Target = [32]byte{2} },
		"record expiry":     func(r *record.Record) { r.RecordNotAfter-- },
		"parent name":       func(r *record.Record) { r.ParentName = "root" },
		"parent generation": func(r *record.Record) { r.ParentGeneration++ },
		"lease expiry":      func(r *record.Record) { r.LeaseExpiresAt++ },
		"grace expiry":      func(r *record.Record) { r.GraceExpiresAt++ },
		"recovery window": func(r *record.Record) {
			r.RecoveryStartedAt++
			r.RecoveryExpiresAt++
		},
		"recovery operation":  func(r *record.Record) { r.RecoveryOperation[0]++ },
		"recovery successor":  func(r *record.Record) { r.RecoverySuccessor[0]++ },
		"recovery policy":     func(r *record.Record) { r.RecoveryPolicy[0]++ },
		"recovery policy rev": func(r *record.Record) { r.RecoveryPolicyRev++ },
		"recovery policy delay": func(r *record.Record) {
			r.RecoveryPolicyDelay++
			r.RecoveryExpiresAt++
		},
		"continuity":          func(r *record.Record) { r.Continuity++ },
		"conflict identifier": func(r *record.Record) { r.ConflictIdentifier = "fork-b" },
	}
	for field, mutate := range mutations {
		field, mutate := field, mutate
		t.Run(field, func(t *testing.T) {
			changed := base
			mutate(&changed)
			wire, err := record.EncodeRecord(changed)
			if err != nil {
				t.Fatalf("mutation did not remain a valid Record: %v", err)
			}
			container := make([]byte, 0, 10+len(wire)+ed25519.SignatureSize)
			container = append(container, signed[:2]...)
			container = binary.BigEndian.AppendUint64(container, uint64(len(wire)))
			container = append(container, wire...)
			container = append(container, signed[len(signed)-ed25519.SignatureSize:]...)
			if _, err := record.VerifyRecord(network, container); err == nil {
				t.Fatal("field mutation retained the original signature")
			}
		})
	}

	wrongLength := append([]byte(nil), signed...)
	binary.BigEndian.PutUint64(wrongLength[2:10], binary.BigEndian.Uint64(wrongLength[2:10])+1)
	if _, err := record.VerifyRecord(network, wrongLength); err == nil {
		t.Fatal("modified container length was accepted")
	}
}

func TestSignRecordRejectsWrongAuthorityAndMalformedKeys(t *testing.T) {
	t.Parallel()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	value := record.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: "not-an-ed25519-key", Target: [32]byte{1},
		LeaseExpiresAt: 200, GraceExpiresAt: 220}
	if _, err := record.SignRecord([32]byte{1}, value, private); err == nil {
		t.Fatal("record was signed for a malformed Authority")
	}
	if _, err := record.SignRecord([32]byte{}, value, private); err == nil {
		t.Fatal("record was signed without a network")
	}
	if _, err := record.VerifyRecord([32]byte{1}, []byte("short")); err == nil {
		t.Fatal("malformed signed record was accepted")
	}
}
