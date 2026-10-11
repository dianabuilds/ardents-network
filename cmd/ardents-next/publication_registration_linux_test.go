//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

// The caller supplies a genuine both-Carrier REGISTER, not a fixture ACK.
// Authority issuance alone is independently encoded fixture input. This checks
// actual host-key/floor/receipt signing, not qualified Publisher or Store readiness.
func exerciseRegisteredInstanceRecord(t *testing.T, registration *introduction.HolderRegistration) func() {
	t.Helper()
	receipt, err := registration.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	network := receipt.Facts().Network
	before := time.Now().UTC().Truncate(time.Second)
	root, err := instance.Initialize(t.Context(), instance.Config{Root: filepath.Join(t.TempDir(), "instance"), NetworkID: network, NotBefore: before, NotAfter: before.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	request, err := root.Request(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	view, err := instance.ParseRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	public := authority[32:]
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	credential := append([]byte{0, 3}, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, view.InstancePublic[:]...)
	for _, value := range []uint64{7, uint64(view.NotBefore), uint64(view.NotAfter)} {
		credential = binary.BigEndian.AppendUint64(credential, value)
	}
	credential = append(credential, network[:]...)
	credential = binary.BigEndian.AppendUint32(credential, 3)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	response := append([]byte("ardents-service-instance-response-v3\x00"), view.Commitment[:]...)
	response = append(response, credential...)
	if _, err = root.Accept(t.Context(), response); err != nil {
		t.Fatal(err)
	}
	history, err := durable.Open(t.Context(), durable.Config{Root: filepath.Join(t.TempDir(), "publication"), Target: target, Network: network})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
		if err := history.Close(); err != nil {
			t.Error(err)
		}
	})
	binding, err := root.Bind(t.Context(), history)
	if err != nil {
		t.Fatal(err)
	}
	record, err := binding.Publication(t.Context(), registration)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := publication.VerifyPublish(record, target, network, time.Now())
	if err != nil || proof.Delegation().Instance != view.InstancePublic || proof.Delegation().Generation != 7 {
		t.Fatal("genuine registration record failed independent verification", err)
	}
	ack := receipt.Facts().Acknowledgement
	commitment := sha256.Sum256(ack[:])
	if !bytes.Equal(record[len("ardents-service-publication-v3\x00")+222:len(record)-64], commitment[:]) {
		t.Fatal("Publication changed canonical actual REGISTER commitment")
	}
	again, err := binding.Publication(t.Context(), registration)
	if err != nil || !bytes.Equal(record, again) {
		t.Fatal("exact receipt retry changed immutable bytes", err)
	}
	recipient, err := binding.NewRecipient(t.Context(), registration)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := recipient.Descriptor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	facts := receipt.Facts()
	verified, err := reachability.Verify(descriptor, target, network, facts.Profile, time.Now())
	if err != nil || verified.Introduction.Node != facts.Node || verified.Introduction.Slot != facts.Slot || verified.Introduction.Revision != facts.Revision || verified.Introduction.RecipientKey == [32]byte{} || verified.Introduction.NotAfter.After(facts.Expiry) || verified.PublicationDigest != proof.Digest() {
		t.Fatal("Descriptor does not bind original genuine REGISTER facts", err)
	}
	retry, err := recipient.Descriptor(t.Context())
	if err != nil || !bytes.Equal(retry, descriptor) {
		t.Fatal("exact Descriptor retry changed key or signed bytes", err)
	}
	if _, err = binding.NewRecipient(t.Context(), registration); err == nil {
		t.Fatal("same real revision created another private recipient")
	}
	expiry := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	if verified.Introduction.NotAfter.Before(expiry) {
		expiry = verified.Introduction.NotAfter
	}
	header := capsule.Header{Slot: facts.Slot, Revision: facts.Revision, Expiry: expiry, DeliveryNonce: [32]byte{21}}
	requestTuple := capsule.Request{Network: network, Target: target, PublicationDigest: proof.Digest(), Revision: facts.Revision,
		RendezvousNode: [32]byte{22}, RendezvousDutyGeneration: 1, JoinSecret: [32]byte{23}, HandshakeContext: [32]byte{24},
		ProfileDigest: facts.Profile, ConnectionNonce: [32]byte{25}, AttachmentGeneration: 1, Deadline: expiry, InitiatorBinding: [32]byte{26},
		WorkSafetyNotAfter: expiry.Unix(), WorkSafetyMaximum: expiry.Unix(), NoNewRecoveryAfter: expiry.Unix()}
	// This tuple is candidate input only; it does not attest Rendezvous,
	// Connection or Work Safety. Actual downstream acceptance remains separate.
	envelope, digest, err := capsule.Seal(header, verified.Introduction.RecipientKey, requestTuple)
	if err != nil {
		t.Fatal(err)
	}
	opening, err := recipient.OpenCapsule(t.Context(), envelope)
	if err != nil {
		t.Fatal("genuine registered recipient opening", err)
	}
	t.Cleanup(opening.Close)
	opened, gotDigest, err := opening.Request(t.Context())
	if err != nil || opened != requestTuple || gotDigest != digest {
		t.Fatal("opening changed exact candidate binding", err)
	}
	for _, field := range []string{"Network", "Target", "Publication", "profile"} {
		changed := requestTuple
		switch field {
		case "Network":
			changed.Network[0] ^= 1
		case "Target":
			changed.Target[0] ^= 1
		case "Publication":
			changed.PublicationDigest[0] ^= 1
		case "profile":
			changed.ProfileDigest[0] ^= 1
		}
		foreign, _, err := capsule.Seal(header, verified.Introduction.RecipientKey, changed)
		if err != nil {
			t.Fatal(err)
		}
		if user, err := recipient.OpenCapsule(t.Context(), foreign); err == nil {
			user.Close()
			t.Fatal("foreign candidate supplied opening", field)
		}
	}
	// The signed expiry remains in the future and the genuine registration is
	// still live. A delayed retirement scheduler must not extend acceptance.
	cutoff := time.Now().Add(25 * time.Millisecond)
	if err := recipient.LimitAcceptance(cutoff); err != nil {
		t.Fatal(err)
	}
	if err := recipient.LimitAcceptance(expiry); err != nil {
		t.Fatal("later limitation changed original outcome", err)
	}
	if err := recipient.LimitAcceptance(time.Time{}); err == nil {
		t.Fatal("zero cutoff supplied an accepting window")
	}
	<-time.NewTimer(time.Until(cutoff)).C
	if err := registration.CheckReceipt(receipt); err != nil {
		t.Fatal("cutoff control lost actual registration", err)
	}
	if !time.Now().Before(expiry) || verified.Current(time.Now()) != nil {
		t.Fatal("cutoff control lost original capsule/Descriptor validity")
	}
	if _, _, err := opening.Request(t.Context()); err == nil {
		t.Fatal("retained opening survived exact local cutoff with live registration")
	}
	if user, err := recipient.OpenCapsule(t.Context(), envelope); err == nil {
		user.Close()
		t.Fatal("new opening survived exact local cutoff with live registration")
	}
	return func() {
		if _, err := binding.Publication(t.Context(), registration); err == nil {
			t.Fatal("retired genuine registration supplied live signing receipt")
		}
		if _, err := recipient.Descriptor(t.Context()); err == nil {
			t.Fatal("retired genuine registration supplied Descriptor authority")
		}
		if _, _, err := opening.Request(t.Context()); err == nil {
			t.Fatal("retired registration supplied retained private candidate")
		}
		// Actual original Instance custody stays retained until the opaque user
		// returns. Root close must interrupt it without joining under its lock.
		joined := make(chan error, 1)
		go func() { joined <- root.Close() }()
		select {
		case <-opening.Context().Done():
		case <-time.After(5 * time.Second):
			t.Fatal("root retirement did not interrupt original opening")
		}
		select {
		case err := <-joined:
			t.Fatal("root returned original custody before opening user joined", err)
		default:
		}
		opening.Close()
		select {
		case err := <-joined:
			if err != nil {
				t.Fatal("root failed original opening join", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("root close joined under a borrower-needed lock")
		}
	}
}
