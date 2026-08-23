package route

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

// EntryAcquirer is the one opaque Entry port needed by the native User-side
// Route opener. Its implementation retains the durable Invite/retry state.
type EntryAcquirer interface {
	Acquire(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error)
}

// EntryAttachmentRequest is the endpoint-selected context for one fresh C-5
// attachment. It contains no Service Target or complete Route selection.
type EntryAttachmentRequest struct {
	NetworkID, Digest, AttachmentID [32]byte
	Epoch                           uint64
	Deadline                        time.Time
}

// OpenEntryAttachment creates one State-pinned native User-to-Initiator TLS
// leg, then writes its exact EntryBinding. Entry owns candidate retries and
// durable contact state; Route owns the fresh attachment identifier and
// binding bytes.
func OpenEntryAttachment(ctx context.Context, source EntryAcquirer, input EntryAttachmentRequest) (net.Conn, func() error, error) {
	if source == nil || input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.Epoch == 0 || input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
		return nil, nil, errors.New("Entry attachment request is invalid")
	}
	return source.Acquire(ctx, entry.Attempt{ID: input.AttachmentID, Deadline: input.Deadline},
		func(contactCtx context.Context, candidate entry.Candidate, presentation entry.Presentation, deadline time.Time) (net.Conn, func() error, bool, error) {
			certificate, err := freshEntryClientCertificate()
			if err != nil {
				return nil, nil, true, err
			}
			secured, err := dialNativeEntryTLS(contactCtx, candidate, certificate, deadline)
			if err != nil {
				return nil, nil, true, err
			}
			cleanup := secured.Close
			digest, err := ClientTLSKeyDigest(certificate.Leaf)
			if err != nil {
				return closedEntryOpener(secured, err)
			}
			binding, err := EncodeEntryBinding(EntryBinding{NetworkID: input.NetworkID, Digest: input.Digest,
				Epoch: input.Epoch, AttachmentID: input.AttachmentID, InitiatorNodeID: candidate.NodeID,
				NotAfter: deadline, ClientKeyDigest: digest, Invite: presentation.Invite})
			if err != nil {
				return closedEntryOpener(secured, err)
			}
			if err := writeAll(secured, binding); err != nil {
				return closedEntryOpener(secured, err)
			}
			return secured, cleanup, true, nil
		})
}

func closedEntryOpener(connection net.Conn, cause error) (net.Conn, func() error, bool, error) {
	closeErr := connection.Close()
	return nil, nil, closeErr == nil, errors.Join(cause, closeErr)
}

func dialNativeEntryTLS(ctx context.Context, candidate entry.Candidate, certificate tls.Certificate, deadline time.Time) (*tls.Conn, error) {
	if candidate.Endpoint == "" || candidate.PublicKey == [32]byte{} || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("Entry candidate is invalid")
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", candidate.Endpoint)
	if err != nil {
		return nil, err
	}
	secured := tls.Client(connection, nativeEntryTLS(certificate, candidate.PublicKey))
	if err := secured.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if secured.ConnectionState().NegotiatedProtocol != routeProfile {
		_ = connection.Close()
		return nil, errors.New("Entry TLS ALPN is invalid")
	}
	return secured, nil
}

func nativeEntryTLS(certificate tls.Certificate, peer [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{routeProfile}, VerifyConnection: exactPeer(peer)}
}

func freshEntryClientCertificate() (tls.Certificate, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{SerialNumber: new(big.Int).SetBytes(public[:8]), Subject: pkix.Name{CommonName: "ardents-entry-attempt"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(16 * time.Minute), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, nil
}
