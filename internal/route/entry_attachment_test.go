package route

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

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
	clientCertificate := entryBindingCertificate(t, 58)
	request := EntryAttachmentRequest{NetworkID: identifier(53), Digest: identifier(54), Epoch: 55,
		AttachmentID: identifier(56), Deadline: deadline, ClientCertificate: clientCertificate}
	presentation := entry.Presentation{InviteID: identifier(57), Invite: []byte{9, 8, 7}}
	serverDone := make(chan error, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		secured := tls.Server(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{serverCertificate}, ClientAuth: tls.RequireAnyClientCert,
			SessionTicketsDisabled: true, NextProtos: []string{Profile}})
		if handshakeErr := secured.HandshakeContext(context.Background()); handshakeErr != nil {
			serverDone <- handshakeErr
			return
		}
		defer secured.Close()
		peers := secured.ConnectionState().PeerCertificates
		if len(peers) != 1 {
			serverDone <- errors.New("entry attachment client certificate is unavailable")
			return
		}
		digest, digestErr := ClientTLSKeyDigest(peers[0])
		if digestErr != nil {
			serverDone <- digestErr
			return
		}
		expected, encodeErr := EncodeEntryBinding(EntryBinding{NetworkID: request.NetworkID, Digest: request.Digest,
			Epoch: request.Epoch, AttachmentID: request.AttachmentID, InitiatorNodeID: candidate.NodeID,
			NotAfter: deadline, ClientKeyDigest: digest, Invite: presentation.Invite})
		if encodeErr != nil {
			serverDone <- encodeErr
			return
		}
		actual := make([]byte, len(expected))
		if _, readErr := io.ReadFull(secured, actual); readErr != nil {
			serverDone <- readErr
			return
		}
		if !bytes.Equal(actual, expected) {
			serverDone <- errors.New("entry attachment wrote a non-canonical binding")
			return
		}
		serverDone <- nil
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
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

type entryAcquirerFunc func(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error)

func (call entryAcquirerFunc) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	return call(ctx, attempt, opener)
}
