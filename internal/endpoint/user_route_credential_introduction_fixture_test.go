package endpoint

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func serveTargetLinkIntroduction(listener net.Listener, network, digest [32]byte, epoch uint64, node [32]byte,
	deadline time.Time, certificate tls.Certificate, outcome byte, onDelivered func(net.Conn) error,
) <-chan error {
	result := make(chan error, 1)
	go func() {
		defer close(result)
		connection, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		accepted, err := route.AcceptEndpointTransitAttachment(context.Background(), connection, route.EndpointTransitAttachmentAcceptance{
			NetworkID: network, Digest: digest, TransitNodeID: node, Epoch: epoch, TransitRole: route.IntroductionRole,
			Deadline: deadline, Certificate: certificate,
			Admit: func(authorization []byte, attachment, key [32]byte, role byte, transit [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
				if len(authorization) == 0 || attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || transit != node || !notAfter.Equal(deadline) {
					return route.EndpointTransitAdmission{}, errors.New("Introduction admission input is invalid")
				}
				return route.EndpointTransitAdmission{AuthorizationID: [32]byte{87}, NetworkID: network, Digest: digest, TransitNodeID: node,
					Epoch: epoch, TransitRole: route.IntroductionRole, NotAfter: deadline}, nil
			}})
		if err != nil {
			result <- err
			return
		}
		defer accepted.Connection.Close()
		control, err := route.ReadIntroductionControlRecord(accepted.Connection)
		if err != nil || control.Sealed == nil {
			result <- errors.Join(errors.New("read sealed Introduction"), err)
			return
		}
		if err := route.WriteIntroductionDeliveryResult(accepted.Connection, route.IntroductionDeliveryResult{AttachmentID: accepted.Binding.AttachmentID, Outcome: outcome}); err != nil {
			result <- err
			return
		}
		if onDelivered != nil {
			result <- onDelivered(accepted.Connection)
			return
		}
		result <- nil
	}()
	return result
}

func targetLinkIntroductionRelayHandler(connection net.Conn, onReady func(net.Conn) error) error {
	setup, err := readInitiatorFixtureRecord(connection, 4)
	if err != nil {
		return err
	}
	if err := writeInitiatorFixtureReady(connection, setup, 5); err != nil {
		return err
	}
	if onReady != nil {
		return onReady(connection)
	}
	_, err = io.Copy(io.Discard, connection)
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
