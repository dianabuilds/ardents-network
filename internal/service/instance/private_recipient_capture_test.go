package instance

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/binary"
	"testing"
	"time"
)

// This models capture of a later live scalar, not capture of the old live key
// or a complete process-memory/P3 experiment. The attacker bypasses owner and
// revision checks and attempts the original authenticated HPKE transcript.
func TestPrivateRecipientLaterKeyCannotOpenRecordedCapsule(t *testing.T) {
	now := time.Date(2030, 4, 5, 6, 7, 8, 0, time.UTC)
	root, err := Initialize(InitializeConfig{Root: instanceFixtureRoot(t), NetworkID: [32]byte{21}, NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	if _, err := root.Accept(issuedResponse(t, root, authority, 1)); err != nil {
		t.Fatal(err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := binding.CommitPublished(1); err != nil {
		t.Fatal(err)
	}
	expiry := now.Add(600 * time.Second)
	first, err := binding.NewPrivateRecipient(1, now, expiry)
	if err != nil {
		t.Fatal(err)
	}
	info := append([]byte(privateIntroductionInfo), bytes.Repeat([]byte{7}, 32)...)
	header := append(append([]byte(nil), info...), make([]byte, 80)...)
	header[len(info)] = 1
	binary.BigEndian.PutUint64(header[len(info)+32:], 1)
	binary.BigEndian.PutUint64(header[len(info)+40:], uint64(expiry.Unix()))
	header[len(info)+48] = 2
	plaintext := make([]byte, 344)
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatal(err)
	}
	defer clear(plaintext)
	seal := func(public [32]byte) ([]byte, []byte) {
		t.Helper()
		ec, err := ecdh.X25519().NewPublicKey(public[:])
		if err != nil {
			t.Fatal(err)
		}
		key, err := hpke.NewDHKEMPublicKey(ec)
		if err != nil {
			t.Fatal(err)
		}
		enc, sender, err := hpke.NewSender(key, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
		if err != nil {
			t.Fatal(err)
		}
		ciphertext, err := sender.Seal(header, plaintext)
		if err != nil {
			t.Fatal(err)
		}
		return enc, ciphertext
	}
	recordedEnc, recordedCiphertext := seal(first.Public(now))
	opened, err := first.OpenPrivateIntroduction(recordedEnc, info, header, recordedCiphertext, now)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("original recipient positive control: %v", err)
	}
	clear(opened)
	cutoff := now.Add(60 * time.Second)
	if err := first.RetainPredecessor(now, cutoff); err != nil {
		t.Fatal(err)
	}
	second, err := binding.NewPrivateRecipient(2, now, expiry)
	if err != nil {
		t.Fatal(err)
	}
	oldScalar := first.private
	if first.Public(cutoff) != [32]byte{} || first.private != nil {
		t.Fatal("predecessor not retired")
	}
	if !bytes.Equal(oldScalar, make([]byte, 32)) {
		t.Fatal("retired scalar not cleared")
	}
	// Test-only capture after retirement; production exposes no scalar accessor.
	captured := append([]byte(nil), second.private...)
	defer clear(captured)
	ec, err := ecdh.X25519().NewPrivateKey(captured)
	if err != nil {
		t.Fatal(err)
	}
	attackerKey, err := hpke.NewDHKEMPrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	attempt := func(enc, ciphertext []byte) ([]byte, error) {
		receiver, err := hpke.NewRecipient(enc, attackerKey, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
		if err != nil {
			return nil, err
		}
		return receiver.Open(header, ciphertext)
	}
	laterEnc, laterCiphertext := seal(second.Public(cutoff))
	opened, err = attempt(laterEnc, laterCiphertext)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("captured later key positive control: %v", err)
	}
	clear(opened)
	opened, err = attempt(recordedEnc, recordedCiphertext)
	if err == nil || len(opened) != 0 {
		t.Fatal("later scalar decrypted recorded predecessor capsule")
	}
	if _, err := first.OpenPrivateIntroduction(recordedEnc, info, header, recordedCiphertext, cutoff); err == nil {
		t.Fatal("retired owner revived")
	}
}
