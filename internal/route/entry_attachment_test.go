package route

import (
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
			SessionTicketsDisabled: true, NextProtos: []string{routeProfile}})
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

type entryAcquirerFunc func(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error)

func (call entryAcquirerFunc) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	return call(ctx, attempt, opener)
}

func readEntryBinding(reader io.Reader) (EntryBinding, error) {
	header := make([]byte, len(routeWireMagic)+2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return EntryBinding{}, err
	}
	length := int(header[len(routeWireMagic)])<<8 | int(header[len(routeWireMagic)+1])
	if length == 0 || length > maximumWireBody {
		return EntryBinding{}, errors.New("Entry binding wire length is invalid")
	}
	raw := append([]byte(nil), header...)
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return EntryBinding{}, err
	}
	raw = append(raw, body...)
	return DecodeEntryBinding(raw)
}
