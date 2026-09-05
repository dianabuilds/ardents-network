package route

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

func entryAdmitterRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "entry-admitter")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestOpenEntryAttachmentUsesStatePinnedTLSAndSendsExactBinding(t *testing.T) {
	serverCertificate := entryBindingCertificate(t, 51)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	public := serverCertificate.Leaf.PublicKey.(ed25519.PublicKey)
	candidate := entry.Candidate{NodeID: identifier(52), Endpoint: listener.Addr().String()}
	copy(candidate.PublicKey[:], public)
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	request := EntryAttachmentRequest{NetworkID: identifier(53), Digest: identifier(54), Epoch: 55,
		AttachmentID: identifier(56), Deadline: deadline}
	presentation := entry.Presentation{InviteID: identifier(57), Invite: []byte{9, 8, 7}}
	received := make(chan EntryBinding, 1)
	failed := make(chan error, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			failed <- acceptErr
			return
		}
		secured := tls.Server(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{serverCertificate}, ClientAuth: tls.RequireAnyClientCert,
			SessionTicketsDisabled: true, NextProtos: []string{Profile}})
		if handshakeErr := secured.HandshakeContext(context.Background()); handshakeErr != nil {
			failed <- handshakeErr
			return
		}
		defer secured.Close()
		binding, readErr := readEntryBinding(secured)
		if readErr != nil {
			failed <- readErr
			return
		}
		received <- binding
	}()
	acquirer := entryAcquirerFunc(func(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
		if attempt.ID != request.AttachmentID || !attempt.Deadline.Equal(deadline) {
			return nil, nil, errors.New("wrong Entry attempt")
		}
		connection, cleanup, complete, openErr := opener(ctx, candidate, presentation, deadline)
		if openErr != nil || !complete {
			return nil, cleanup, openErr
		}
		return connection, cleanup, nil
	})
	connection, cleanup, err := OpenEntryAttachment(t.Context(), acquirer, request)
	if err != nil || connection == nil || cleanup == nil {
		t.Fatalf("OpenEntryAttachment returned connection=%v cleanup-present=%t err=%v", connection, cleanup != nil, err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-failed:
		t.Fatal(err)
	case binding := <-received:
		if binding.NetworkID != request.NetworkID || binding.Digest != request.Digest || binding.Epoch != request.Epoch ||
			binding.AttachmentID != request.AttachmentID || binding.InitiatorNodeID != candidate.NodeID ||
			!binding.NotAfter.Equal(deadline) || string(binding.Invite) != string(presentation.Invite) {
			t.Fatalf("EntryBinding = %+v", binding)
		}
		digest, digestErr := ClientTLSKeyDigest(serverCertificate.Leaf)
		if digestErr == nil && binding.ClientKeyDigest == digest {
			t.Fatal("EntryBinding used the Initiator certificate as its client key")
		}
	}
}

func TestAcceptEntryAttachmentVerifiesAndConsumesBeforeReturning(t *testing.T) {
	serverCertificate := entryBindingCertificate(t, 61)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	public := serverCertificate.Leaf.PublicKey.(ed25519.PublicKey)
	candidate := entry.Candidate{NodeID: identifier(62), Endpoint: listener.Addr().String()}
	copy(candidate.PublicKey[:], public)
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	request := EntryAttachmentRequest{NetworkID: identifier(63), Digest: identifier(64), Epoch: 65,
		AttachmentID: identifier(66), Deadline: deadline}
	presentation := entry.Presentation{InviteID: identifier(67), Invite: []byte{1, 3, 3, 7}}
	verified := make(chan struct{}, 1)
	consumed := make(chan struct{}, 1)
	serverDone := make(chan error, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		admission := EntryAdmission{InviteID: presentation.InviteID, NetworkID: request.NetworkID, Digest: request.Digest,
			Epoch: request.Epoch, InitiatorNodeID: candidate.NodeID, NotAfter: deadline}
		secured, acceptErr := AcceptEntryAttachment(t.Context(), raw, EntryAttachmentAcceptance{
			NetworkID: request.NetworkID, Digest: request.Digest, Epoch: request.Epoch, InitiatorNodeID: candidate.NodeID,
			Deadline: deadline, Certificate: serverCertificate,
			Admit: func(invite []byte, attachment, key [32]byte, notAfter time.Time) (EntryAdmission, error) {
				if string(invite) != string(presentation.Invite) {
					return EntryAdmission{}, errors.New("wrong Invite")
				}
				if attachment != request.AttachmentID || key == [32]byte{} || !notAfter.Equal(deadline) {
					return EntryAdmission{}, errors.New("wrong consumed Entry tuple")
				}
				verified <- struct{}{}
				consumed <- struct{}{}
				return admission, nil
			}})
		if secured != nil {
			_ = secured.Close()
		}
		serverDone <- acceptErr
	}()
	acquirer := entryAcquirerFunc(func(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
		connection, cleanup, complete, openErr := opener(ctx, candidate, presentation, deadline)
		if openErr != nil || !complete {
			return nil, cleanup, openErr
		}
		return connection, cleanup, nil
	})
	connection, cleanup, err := OpenEntryAttachment(t.Context(), acquirer, request)
	if err != nil || connection == nil || cleanup == nil {
		t.Fatalf("OpenEntryAttachment returned connection=%v cleanup-present=%t err=%v", connection, cleanup != nil, err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-verified:
	default:
		t.Fatal("Entry Invite was not verified")
	}
	select {
	case <-consumed:
	default:
		t.Fatal("Entry replay tuple was not consumed")
	}
}

func TestAcceptEntryAttachmentRefusesAdmissionDeadlinePastDutyExpiry(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	_, err := AcceptEntryAttachment(t.Context(), server, EntryAttachmentAcceptance{NetworkID: identifier(68), Digest: identifier(69),
		InitiatorNodeID: identifier(70), Epoch: 71, Deadline: deadline, AdmissionDeadline: deadline.Add(time.Second),
		Certificate: entryBindingCertificate(t, 72), Admit: func([]byte, [32]byte, [32]byte, time.Time) (EntryAdmission, error) {
			return EntryAdmission{}, nil
		}})
	if err == nil {
		t.Fatal("Entry attachment accepted an admission deadline beyond its duty expiry")
	}
}

func TestEntryAdmitterPortUsesOneDurableEntryOperation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	candidate := entry.Candidate{NodeID: identifier(75), KeyID: identifier(76), FamilyID: identifier(77),
		RecordDigest: identifier(78), DomainProofDigest: identifier(79), Endpoint: "127.0.0.1:7999", Capacity: 1,
		Domain: "initiator", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), AssignmentNotAfter: now.Add(time.Hour)}
	copy(candidate.PublicKey[:], public)
	view := entry.View{NetworkID: identifier(80), Epoch: 81, Digest: identifier(82), Profile: Profile, Fresh: true,
		Candidates: []entry.Candidate{candidate}}
	verification := entry.Verification{Current: func() (entry.View, error) { return view, nil },
		Conflict: func([32]byte, [32]byte) (bool, error) { return false, nil }, Clock: func() time.Time { return now }, TimeConfident: func() bool { return true }}
	admitter, err := entry.OpenAdmitter(entry.AdmitterConfig{Root: entryAdmitterRoot(t), Verification: verification})
	if err != nil {
		t.Fatal(err)
	}
	defer admitter.Close()
	raw := routeTestInvite(view, candidate, private, now)
	port := EntryAdmitterPort(admitter)
	if port == nil {
		t.Fatal("Entry Admitter port is unavailable")
	}
	attachment, clientKey := identifier(83), identifier(84)
	admission, err := port(raw, attachment, clientKey, now.Add(time.Minute))
	if err != nil || admission.NetworkID != view.NetworkID || admission.Digest != view.Digest || admission.Epoch != view.Epoch ||
		admission.InitiatorNodeID != candidate.NodeID {
		t.Fatalf("Entry Admitter port = %+v, %v", admission, err)
	}
	if _, err := port(raw, attachment, clientKey, now.Add(time.Minute)); err == nil {
		t.Fatal("Entry Admitter port accepted a replayed tuple")
	}
}

type entryAcquirerFunc func(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error)

func (call entryAcquirerFunc) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	return call(ctx, attempt, opener)
}

func routeTestInvite(view entry.View, candidate entry.Candidate, private ed25519.PrivateKey, now time.Time) []byte {
	body := make([]byte, 0, 256)
	body = appendUint16(body, 2)
	body = append(body, view.NetworkID[:]...)
	body = appendUint64(body, view.Epoch)
	body = append(body, view.Digest[:]...)
	body = append(body, byte(len(Profile)))
	body = append(body, Profile...)
	body = append(body, candidate.KeyID[:]...)
	body = append(body, candidate.NodeID[:]...)
	body = append(body, candidate.FamilyID[:]...)
	body = append(body, candidate.RecordDigest[:]...)
	body = append(body, candidate.DomainProofDigest[:]...)
	body = appendUint64(body, uint64(candidate.AssignmentNotAfter.Unix()))
	body = appendUint64(body, uint64(now.Add(-time.Minute).Unix()))
	body = appendUint64(body, uint64(now.Add(30*time.Minute).Unix()))
	body = append(body, 1, 0, 0)
	signature := ed25519.Sign(private, append([]byte("ardents-entry-invite-signature-v2\x00"), body...))
	raw := make([]byte, 0, len("ardents-entry-invite-v2")+2+len(body)+len(signature))
	raw = append(raw, "ardents-entry-invite-v2"...)
	raw = appendUint16(raw, uint16(len(body)))
	raw = append(raw, body...)
	return append(raw, signature...)
}
