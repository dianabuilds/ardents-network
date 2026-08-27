//go:build ignore

// This throwaway R-118 logic prototype is invoked by file path. It is not a
// maintained root-module package or a Credential Relay implementation.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type result struct {
	UserImport        entry.Class `json:"user_import"`
	AttachmentAccept  bool        `json:"attachment_accepted"`
	OpaqueReceived    bool        `json:"opaque_received"`
	InviteInPostInput bool        `json:"invite_in_post_input"`
}

func main() {
	value, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, _ := json.Marshal(value)
	fmt.Println(string(raw))
}

func run() (result, error) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(30 * time.Second)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return result{}, err
	}
	defer listener.Close()
	seed := identifier("r118-entry-initiator")
	serverPrivate := ed25519.NewKeyFromSeed(seed[:])
	serverCertificate, err := certificate(serverPrivate, 1, now)
	if err != nil {
		return result{}, err
	}
	var serverPublic [32]byte
	copy(serverPublic[:], serverPrivate.Public().(ed25519.PublicKey))
	view := entry.View{NetworkID: identifier("entry-network"), Epoch: 1, Digest: identifier("entry-digest"), Profile: route.Profile, Fresh: true}
	candidate := entry.Candidate{NodeID: identifier("entry-initiator"), PublicKey: serverPublic, KeyID: identifier("entry-key"),
		FamilyID: identifier("entry-family"), RecordDigest: identifier("entry-record"), DomainProofDigest: identifier("entry-domain"),
		Endpoint: listener.Addr().String(), Capacity: 1, Domain: "initiator", ValidFrom: now.Add(-time.Second), ValidUntil: deadline,
		AssignmentNotAfter: deadline}
	view.Candidates = []entry.Candidate{candidate}
	userRoot, err := os.MkdirTemp("", "ardents-r118-entry-user-")
	if err != nil {
		return result{}, err
	}
	defer os.RemoveAll(userRoot)
	initiatorRoot, err := os.MkdirTemp("", "ardents-r118-entry-initiator-")
	if err != nil {
		return result{}, err
	}
	defer os.RemoveAll(initiatorRoot)
	current := func() (entry.View, error) { return view, nil }
	conflict := func([32]byte, [32]byte) (bool, error) { return false, nil }
	config := entry.Config{Root: userRoot, Current: current, Conflict: conflict, Clock: func() time.Time { return time.Now().UTC() }, TimeConfident: func() bool { return true }}
	user, err := entry.Open(config)
	if err != nil {
		return result{}, err
	}
	defer user.Close()
	admitter, err := entry.OpenAdmitter(entry.AdmitterConfig{Root: initiatorRoot, Verification: entry.Verification{Current: current, Conflict: conflict,
		Clock: func() time.Time { return time.Now().UTC() }, TimeConfident: func() bool { return true }}})
	if err != nil {
		return result{}, err
	}
	defer admitter.Close()
	invite, err := entry.Issue(entry.IssueInput{NetworkID: view.NetworkID, Digest: view.Digest, Epoch: view.Epoch, Candidate: candidate,
		NotBefore: now.Add(-time.Second), NotAfter: deadline, Slot: 0, Generation: 1}, serverPrivate)
	if err != nil {
		return result{}, err
	}
	imported, err := user.Import(invite)
	if err != nil || imported.Class != entry.Accepted {
		return result{UserImport: imported.Class}, errors.Join(err, errors.New("Entry Invite was not accepted by User owner"))
	}
	opaque := make([]byte, 32)
	if _, err := rand.Read(opaque); err != nil {
		return result{}, err
	}
	attachment := identifier("r118-entry-attachment")
	accepted := make(chan result, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			accepted <- result{UserImport: imported.Class}
			return
		}
		connection, acceptErr := route.AcceptEntryAttachment(context.Background(), raw, route.EntryAttachmentAcceptance{NetworkID: view.NetworkID,
			Digest: view.Digest, Epoch: view.Epoch, InitiatorNodeID: candidate.NodeID, Deadline: deadline, Certificate: serverCertificate,
			Admit: route.EntryAdmitterPort(admitter)})
		if acceptErr != nil {
			accepted <- result{UserImport: imported.Class}
			return
		}
		defer connection.Close()
		received := make([]byte, len(opaque))
		_, readErr := io.ReadFull(connection, received)
		accepted <- result{UserImport: imported.Class, AttachmentAccept: true, OpaqueReceived: readErr == nil && bytes.Equal(received, opaque),
			InviteInPostInput: bytes.Contains(received, invite)}
	}()
	connection, cleanup, err := route.OpenEntryAttachment(context.Background(), user, route.EntryAttachmentRequest{NetworkID: view.NetworkID,
		Digest: view.Digest, Epoch: view.Epoch, AttachmentID: attachment, Deadline: deadline})
	if err != nil {
		return result{UserImport: imported.Class}, err
	}
	if _, err := connection.Write(opaque); err != nil {
		_ = cleanup()
		return result{UserImport: imported.Class}, err
	}
	if err := cleanup(); err != nil {
		return result{UserImport: imported.Class}, err
	}
	value := <-accepted
	if !value.AttachmentAccept || !value.OpaqueReceived || value.InviteInPostInput {
		return value, errors.New("real Entry carrier prototype failed")
	}
	return value, nil
}

func certificate(private ed25519.PrivateKey, serial int64, now time.Time) (tls.Certificate, error) {
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "r118-entry-carrier"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, nil
}

func identifier(label string) [32]byte { return sha256.Sum256([]byte(label)) }
