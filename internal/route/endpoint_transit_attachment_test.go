package route

import (
	"crypto/ed25519"
	"errors"
	"net"
	"testing"
	"time"
)

func TestEndpointTransitAttachmentVerifiesAndConsumesExactAuthorization(t *testing.T) {
	serverCertificate := entryBindingCertificate(t, 91)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var serverPublic [32]byte
	copy(serverPublic[:], serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	request := EndpointTransitAttachmentRequest{NetworkID: identifier(92), Digest: identifier(93), AttachmentID: identifier(94),
		TransitNodeID: identifier(95), TransitNodePublicKey: serverPublic, Epoch: 96, TransitRole: IntroductionRole,
		Endpoint: listener.Addr().String(), Deadline: deadline, Authorization: []byte{4, 5, 6}}
	admitted := make(chan struct{}, 1)
	serverDone := make(chan error, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		accepted, acceptErr := AcceptEndpointTransitAttachment(t.Context(), raw, EndpointTransitAttachmentAcceptance{
			NetworkID: request.NetworkID, Digest: request.Digest, TransitNodeID: request.TransitNodeID, Epoch: request.Epoch,
			TransitRole: request.TransitRole, Deadline: deadline, Certificate: serverCertificate,
			Admit: func(authorization []byte, attachment, key [32]byte, role byte, node [32]byte, notAfter time.Time) (EndpointTransitAdmission, error) {
				if string(authorization) != string(request.Authorization) || attachment != request.AttachmentID || key == [32]byte{} ||
					role != request.TransitRole || node != request.TransitNodeID || !notAfter.Equal(deadline) {
					return EndpointTransitAdmission{}, errors.New("unexpected transit authorization")
				}
				admitted <- struct{}{}
				return EndpointTransitAdmission{AuthorizationID: identifier(97), NetworkID: request.NetworkID, Digest: request.Digest,
					Epoch: request.Epoch, TransitRole: request.TransitRole, TransitNodeID: request.TransitNodeID, NotAfter: deadline}, nil
			}})
		if accepted.Connection != nil {
			if accepted.Binding.AttachmentID != request.AttachmentID || accepted.Binding.Authorization == nil {
				serverDone <- errors.New("accepted transit binding was not returned")
				return
			}
			_ = accepted.Connection.Close()
		}
		serverDone <- acceptErr
	}()
	connection, err := OpenEndpointTransitAttachment(t.Context(), request)
	if err != nil || connection == nil {
		t.Fatalf("OpenEndpointTransitAttachment = %v, %v", connection, err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-admitted:
	default:
		t.Fatal("transit authorization was not consumed")
	}
}
