package route

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"time"
)

// EndpointTransitAttachmentRequest is the local Endpoint-selected context for
// one direct adjacent Introduction or Responder TLS attempt. Endpoint is local
// State material and never appears in its binding record.
type EndpointTransitAttachmentRequest struct {
	NetworkID, Digest, AttachmentID, TransitNodeID, TransitNodePublicKey [32]byte
	Epoch                                                                uint64
	TransitRole                                                          byte
	Endpoint                                                             string
	Deadline                                                             time.Time
	Authorization                                                        []byte
}

// EndpointTransitAttachmentAcceptance is the receiving transit duty's narrow
// State/authorization port. It has no Route plan, Target, or Service material.
type EndpointTransitAttachmentAcceptance struct {
	NetworkID, Digest, TransitNodeID [32]byte
	Epoch                            uint64
	TransitRole                      byte
	Deadline                         time.Time
	Certificate                      tls.Certificate
	Admit                            EndpointTransitBindingAdmitter
}

// AcceptedEndpointTransitAttachment is the authenticated TLS byte carrier and
// its exact, already consumed binding. The receiving duty uses the binding to
// classify the first closed control record; it must not recover identity from
// untrusted control bytes.
type AcceptedEndpointTransitAttachment struct {
	Connection net.Conn
	Binding    EndpointTransitBinding
}

// OpenEndpointTransitAttachment creates one fresh mutually authenticated TLS
// attempt and writes its exact EndpointTransitBinding.
func OpenEndpointTransitAttachment(ctx context.Context, input EndpointTransitAttachmentRequest) (net.Conn, error) {
	if ctx == nil || input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.TransitNodeID == [32]byte{} || input.TransitNodePublicKey == [32]byte{} || input.Epoch == 0 ||
		(input.TransitRole != IntroductionRole && input.TransitRole != ResponderRole) || !literalEndpoint(input.Endpoint) ||
		input.Deadline.IsZero() || !time.Now().Before(input.Deadline) || len(input.Authorization) == 0 ||
		len(input.Authorization) > maximumTransitAuthorization {
		return nil, errors.New("endpoint transit attachment request is invalid")
	}
	certificate, err := freshEndpointClientCertificate()
	if err != nil {
		return nil, err
	}
	secured, err := dialNativeEndpointTLS(ctx, input.Endpoint, input.TransitNodePublicKey, certificate, input.Deadline)
	if err != nil {
		return nil, err
	}
	digest, err := ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		return closeTransitAttachment(secured, err)
	}
	binding, err := EncodeEndpointTransitBinding(EndpointTransitBinding{NetworkID: input.NetworkID, Digest: input.Digest,
		AttachmentID: input.AttachmentID, TransitNodeID: input.TransitNodeID, Epoch: input.Epoch, TransitRole: input.TransitRole,
		NotAfter: input.Deadline.UTC().Truncate(time.Second), ClientKeyDigest: digest, Authorization: input.Authorization})
	if err != nil {
		return closeTransitAttachment(secured, err)
	}
	if err := writeAll(secured, binding); err != nil {
		return closeTransitAttachment(secured, err)
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		return closeTransitAttachment(secured, err)
	}
	return secured, nil
}

// AcceptEndpointTransitAttachment handshakes, validates the closed binding,
// and atomically consumes its opaque authorization before returning bytes.
func AcceptEndpointTransitAttachment(ctx context.Context, connection net.Conn, input EndpointTransitAttachmentAcceptance) (AcceptedEndpointTransitAttachment, error) {
	if ctx == nil || connection == nil || input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.TransitNodeID == [32]byte{} ||
		input.Epoch == 0 || (input.TransitRole != IntroductionRole && input.TransitRole != ResponderRole) || input.Deadline.IsZero() ||
		input.Certificate.PrivateKey == nil || input.Admit == nil || !time.Now().Before(input.Deadline) {
		return AcceptedEndpointTransitAttachment{}, errors.New("endpoint transit attachment acceptance is invalid")
	}
	secured := tls.Server(connection, nativeEndpointTransitTLS(input.Certificate))
	if err := secured.SetDeadline(input.Deadline); err != nil {
		_ = connection.Close()
		return AcceptedEndpointTransitAttachment{}, err
	}
	if err := secured.HandshakeContext(ctx); err != nil || secured.ConnectionState().NegotiatedProtocol != Profile {
		_ = connection.Close()
		return AcceptedEndpointTransitAttachment{}, errors.New("endpoint transit TLS handshake is invalid")
	}
	binding, err := readEndpointTransitBinding(secured)
	if err != nil || binding.NetworkID != input.NetworkID || binding.Digest != input.Digest || binding.Epoch != input.Epoch ||
		binding.TransitRole != input.TransitRole || binding.TransitNodeID != input.TransitNodeID || binding.NotAfter.After(input.Deadline) {
		_ = connection.Close()
		return AcceptedEndpointTransitAttachment{}, errors.New("endpoint transit binding does not match transit duty")
	}
	peers := secured.ConnectionState().PeerCertificates
	if len(peers) != 1 {
		_ = connection.Close()
		return AcceptedEndpointTransitAttachment{}, errors.New("endpoint transit TLS client certificate is unavailable")
	}
	digest, err := ClientTLSKeyDigest(peers[0])
	if err != nil || AdmitEndpointTransitBinding(binding, digest, time.Now().UTC(), input.Admit) != nil {
		_ = connection.Close()
		return AcceptedEndpointTransitAttachment{}, errors.New("endpoint transit binding was refused")
	}
	if err := secured.SetDeadline(time.Time{}); err != nil {
		_ = connection.Close()
		return AcceptedEndpointTransitAttachment{}, err
	}
	return AcceptedEndpointTransitAttachment{Connection: secured, Binding: binding}, nil
}

func closeTransitAttachment(connection net.Conn, cause error) (net.Conn, error) {
	return nil, errors.Join(cause, connection.Close())
}

func readEndpointTransitBinding(reader io.Reader) (EndpointTransitBinding, error) {
	header := make([]byte, len(routeWireMagic)+2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return EndpointTransitBinding{}, err
	}
	length := int(header[len(routeWireMagic)])<<8 | int(header[len(routeWireMagic)+1])
	if length == 0 || length > maximumWireBody {
		return EndpointTransitBinding{}, errors.New("endpoint transit binding wire length is invalid")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return EndpointTransitBinding{}, err
	}
	return DecodeEndpointTransitBinding(append(header, body...))
}
