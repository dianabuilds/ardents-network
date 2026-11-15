//go:build linux && introduction_installed_native

package introduction_test

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	routeintroduction "github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

// Independent adjacent-peer protocol controls, not a live Publication or Service.
// The installed receiver supplies all successful spend, Registry and delivery
// effects; this recipient verifies only unchanged sealed transport and plaintext.
func runIntroductionNativeOperations(t *testing.T, prepared introductionPreparedStock, registrationCertificate, submissionCertificate tls.Certificate, spendRoot, historyRoot string) {
	t.Helper()
	read := func(root, name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	initialSpend := read(spendRoot, "closed-token-spends")
	initialHistory := read(historyRoot, "closed-introduction-slots")
	if len(initialSpend) != 112 || string(initialSpend[:8]) != "ARDSPN01" || len(initialHistory) != 120 || string(initialHistory[:8]) != "ARDISL01" {
		t.Fatal("genuine empty original histories required")
	}
	var slots [][32]byte
	for revision := uint64(1); revision <= 2; revision++ {
		registration := openIntroductionNativeChannel(t, prepared, registrationCertificate, uint8(ardp.PurposeIntroduction))
		slot := introductionNativeNonce(t)
		slots = append(slots, slot)
		register := routeintroduction.Request{Nonce: introductionNativeNonce(t), Slot: slot, Revision: revision, Expiry: registration.deadline}
		body, err := routeintroduction.EncodeRequest(register)
		if err != nil {
			t.Fatal(err)
		}
		introductionNativeSend(t, registration, ardp.Frame{Kind: ardp.KindOperation, Body: body})
		introductionNativeResult(t, registration, register.Nonce, 0)
		private, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		var public [32]byte
		copy(public[:], private.PublicKey().Bytes())
		submission := openIntroductionNativeChannel(t, prepared, submissionCertificate, uint8(ardp.PurposeSubmission))
		expiry := submission.deadline
		request := capsule.Request{Network: prepared.facts.NetworkID, ProfileDigest: prepared.facts.Digest, Target: introductionNativeNonce(t), PublicationDigest: introductionNativeNonce(t), Revision: revision, RendezvousNode: introductionNativeNonce(t), RendezvousDutyGeneration: 1, JoinSecret: introductionNativeNonce(t), HandshakeContext: introductionNativeNonce(t), ConnectionNonce: introductionNativeNonce(t), AttachmentGeneration: 1, Deadline: expiry, InitiatorBinding: introductionNativeNonce(t), WorkSafetyNotAfter: expiry.Unix(), WorkSafetyMaximum: expiry.Unix(), NoNewRecoveryAfter: expiry.Unix()}
		envelope, _, err := capsule.Seal(capsule.Header{Slot: slot, Revision: revision, Expiry: expiry, DeliveryNonce: introductionNativeNonce(t)}, public, request)
		if err != nil {
			t.Fatal(err)
		}
		nonce := introductionNativeNonce(t)
		body, err = routeintroduction.EncodeDelivery(nonce, envelope)
		if err != nil {
			t.Fatal(err)
		}
		introductionNativeSend(t, submission, ardp.Frame{Kind: ardp.KindOperation, Body: body})
		delivered, err := ardp.ReadFrame(registration.conn)
		if err != nil || delivered.Kind != ardp.KindOperation || delivered.Lane == 0 || delivered.Lane%2 != 0 || len(delivered.Body) != 4096 || delivered.Body[0] != 4 || !bytes.Equal(delivered.Body[33:507], envelope.Bytes()) || !bytes.Equal(delivered.Body[507:], make([]byte, 3589)) {
			t.Fatal("original opaque delivery differs", err)
		}
		var deliveryNonce [32]byte
		copy(deliveryNonce[:], delivered.Body[1:33])
		if deliveryNonce == nonce || deliveryNonce == envelope.Header().DeliveryNonce || deliveryNonce == ([32]byte{}) {
			t.Fatal("delivery nonce crossed channel boundaries")
		}
		if err := introductionNativeOpenCapsule(private, prepared.facts.Digest, envelope.Bytes(), request); err != nil {
			t.Fatal(err)
		}
		result, err := routeintroduction.EncodeResult(deliveryNonce, 0)
		if err != nil {
			t.Fatal(err)
		}
		introductionNativeSend(t, registration, ardp.Frame{Kind: ardp.KindResult, Lane: delivered.Lane, Body: result})
		closed, err := ardp.ReadFrame(registration.conn)
		if err != nil || closed.Kind != ardp.KindClose || closed.Lane != delivered.Lane || !bytes.Equal(closed.Body, []byte{0}) {
			t.Fatal("delivery RESULT did not join matching CLOSE", err)
		}
		introductionNativeResult(t, submission, nonce, 0)
		if err := submission.finish(t.Context()); err != nil {
			t.Fatal("original submission completion", err)
		}
		withdraw := routeintroduction.Request{Nonce: introductionNativeNonce(t), Slot: slot, Revision: revision, Withdraw: true}
		body, err = routeintroduction.EncodeRequest(withdraw)
		if err != nil {
			t.Fatal(err)
		}
		introductionNativeSend(t, registration, ardp.Frame{Kind: ardp.KindOperation, Body: body})
		introductionNativeResult(t, registration, withdraw.Nonce, 0)
		if err := registration.finish(t.Context()); err != nil {
			t.Fatal("original owning withdrawal completion", err)
		}
		t.Logf("revision %d: REGISTER, unchanged sealed delivery, matching RESULT/CLOSE and owning WITHDRAW physically joined", revision)
	}
	spends, history := read(spendRoot, "closed-token-spends"), read(historyRoot, "closed-introduction-slots")
	if len(spends) != len(initialSpend)+4*41 || !bytes.Equal(spends[:112], initialSpend) {
		t.Fatal("four original irreversible spends required")
	}
	if len(history) != 120+2*40 || !bytes.Equal(history[:112], initialHistory[:112]) {
		t.Fatal("two original durable slot floors required")
	}
	for _, slot := range slots {
		digest := sha256.Sum256(slot[:])
		found := false
		for offset := 120; offset < len(history); offset += 40 {
			if bytes.Equal(history[offset:offset+32], digest[:]) && binary.BigEndian.Uint64(history[offset+32:offset+40]) > uint64(time.Now().Unix()) {
				found = true
			}
		}
		if !found {
			t.Fatal("withdrawal reclaimed original slot floor")
		}
	}
}

func introductionNativeNonce(t *testing.T) (nonce [32]byte) {
	t.Helper()
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	return nonce
}

func introductionNativeSend(t *testing.T, c *introductionNativeChannel, frame ardp.Frame) {
	t.Helper()
	if err := c.check(); err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(c.conn, frame); err != nil {
		t.Fatal("original frame output", err)
	}
	if err := c.check(); err != nil {
		t.Fatal(err)
	}
}

func introductionNativeResult(t *testing.T, c *introductionNativeChannel, nonce [32]byte, want uint8) {
	t.Helper()
	frame, err := ardp.ReadFrame(c.conn)
	if err != nil || frame.Kind != ardp.KindResult || frame.Lane != 0 {
		t.Fatal("original RESULT missing", err)
	}
	status, err := routeintroduction.DecodeResult(frame.Body, nonce)
	if err != nil || status != want {
		t.Fatal("original RESULT differs", status, want, err)
	}
	if err := c.check(); err != nil {
		t.Fatal(err)
	}
}

func introductionNativeOpenCapsule(private *ecdh.PrivateKey, profile [32]byte, raw []byte, expected capsule.Request) error {
	key, err := hpke.NewDHKEMPrivateKey(private)
	if err != nil {
		return err
	}
	info := append([]byte("ardents-introduction-capsule-v3\x00"), profile[:]...)
	recipient, err := hpke.NewRecipient(raw[80:112], key, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return err
	}
	plain, err := recipient.Open(append(append([]byte(nil), info...), raw[:80]...), raw[114:])
	if err != nil {
		return err
	}
	defer clear(plain)
	request, err := capsule.ParseRequest(plain)
	if err != nil || request != expected {
		return errors.Join(errors.New("isolated recipient plaintext differs"), err)
	}
	return nil
}
