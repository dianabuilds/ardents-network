package route

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
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

// EntryAttachmentAcceptance is the Initiator's narrow state and replay port.
// Its verifier and consumer are supplied by the owning Entry/State
// composition; Route never receives a State root or durable replay map.
type EntryAttachmentAcceptance struct {
	NetworkID, Digest, InitiatorNodeID [32]byte
	Epoch                              uint64
	Deadline                           time.Time
	Certificate                        tls.Certificate
	Admit                              EntryBindingAdmitter
}

// EntryAdmitterPort adapts Entry's durable one-operation admission to Route's
// opaque port. It exposes no Entry root, candidate, User identifier, or raw
// State fact to the Route accept loop.
func EntryAdmitterPort(value *entry.Admitter) EntryBindingAdmitter {
	if value == nil {
		return nil
	}
	return func(invite []byte, attachment, clientKey [32]byte, notAfter time.Time) (EntryAdmission, error) {
		authorization, err := value.AdmitAndConsume(invite, attachment, clientKey, notAfter)
		if err != nil {
			return EntryAdmission{}, err
		}
		return EntryAdmission{InviteID: authorization.InviteID, NetworkID: authorization.NetworkID, Digest: authorization.Digest,
			Epoch: authorization.Epoch, InitiatorNodeID: authorization.InitiatorNodeID, NotAfter: authorization.NotAfter}, nil
	}
}

// OpenEntryAttachment creates one State-pinned native User-to-Initiator TLS
// leg, then writes its exact EntryBinding. Entry owns candidate retries and
// durable contact state; Route owns the fresh attachment identifier and
// binding bytes.
func OpenEntryAttachment(ctx context.Context, source EntryAcquirer, input EntryAttachmentRequest) (net.Conn, func() error, error) {
	if source == nil || input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.Epoch == 0 || input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
		return nil, nil, errors.New("entry attachment request is invalid")
	}
	return source.Acquire(ctx, entry.Attempt{ID: input.AttachmentID, Deadline: input.Deadline},
		func(contactCtx context.Context, candidate entry.Candidate, presentation entry.Presentation, deadline time.Time) (net.Conn, func() error, bool, error) {
			certificate, err := freshEndpointClientCertificate()
			if err != nil {
				return nil, nil, true, err
			}
			secured, err := dialNativeEndpointTLS(contactCtx, candidate.Endpoint, candidate.PublicKey, certificate, deadline)
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

// AcceptEntryAttachment performs the native Initiator-side TLS handshake,
// reads one EntryBinding, and consumes its replay tuple before it returns the
// usable attachment. It leaves no Route work allocated on refusal.
func AcceptEntryAttachment(ctx context.Context, connection net.Conn, input EntryAttachmentAcceptance) (net.Conn, error) {
	if connection == nil || input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.InitiatorNodeID == [32]byte{} ||
		input.Epoch == 0 || input.Deadline.IsZero() || input.Certificate.PrivateKey == nil || input.Admit == nil ||
		!time.Now().Before(input.Deadline) {
		return nil, errors.New("entry attachment acceptance is invalid")
	}
	secured := tls.Server(connection, nativeEndpointTransitTLS(input.Certificate))
	if err := secured.SetDeadline(input.Deadline); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if secured.ConnectionState().NegotiatedProtocol != Profile {
		_ = connection.Close()
		return nil, errors.New("entry TLS ALPN is invalid")
	}
	binding, err := readEntryBinding(secured)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	if binding.NetworkID != input.NetworkID || binding.Digest != input.Digest || binding.Epoch != input.Epoch ||
		binding.InitiatorNodeID != input.InitiatorNodeID || binding.NotAfter.After(input.Deadline) {
		_ = connection.Close()
		return nil, errors.New("entry binding does not match Initiator duty")
	}
	peer := secured.ConnectionState().PeerCertificates
	if len(peer) != 1 {
		_ = connection.Close()
		return nil, errors.New("entry TLS client certificate is unavailable")
	}
	if err := AdmitEntryBinding(binding, peer[0], time.Now().UTC(), input.Admit); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return secured, nil
}

func closedEntryOpener(connection net.Conn, cause error) (net.Conn, func() error, bool, error) {
	closeErr := connection.Close()
	return nil, nil, closeErr == nil, errors.Join(cause, closeErr)
}

func dialNativeEndpointTLS(ctx context.Context, endpoint string, peer [32]byte, certificate tls.Certificate, deadline time.Time) (*tls.Conn, error) {
	if !literalEndpoint(endpoint) || peer == [32]byte{} || deadline.IsZero() || !time.Now().Before(deadline) {
		return nil, errors.New("entry candidate is invalid")
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return nil, err
	}
	secured := tls.Client(connection, nativeEndpointTLS(certificate, peer))
	if err := secured.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if secured.ConnectionState().NegotiatedProtocol != Profile {
		_ = connection.Close()
		return nil, errors.New("entry TLS ALPN is invalid")
	}
	return secured, nil
}

func nativeEndpointTLS(certificate tls.Certificate, peer [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{Profile}, VerifyConnection: exactPeer(peer)}
}

func nativeEndpointTransitTLS(certificate tls.Certificate) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.RequireAnyClientCert, SessionTicketsDisabled: true, NextProtos: []string{Profile}}
}

func freshEndpointClientCertificate() (tls.Certificate, error) {
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

func readEntryBinding(reader io.Reader) (EntryBinding, error) {
	header := make([]byte, len(routeWireMagic)+2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return EntryBinding{}, err
	}
	length := int(header[len(routeWireMagic)])<<8 | int(header[len(routeWireMagic)+1])
	if length == 0 || length > maximumWireBody {
		return EntryBinding{}, errors.New("entry binding wire length is invalid")
	}
	raw := append([]byte(nil), header...)
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return EntryBinding{}, err
	}
	return DecodeEntryBinding(append(raw, body...))
}
