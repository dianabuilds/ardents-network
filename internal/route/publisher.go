package route

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

func servePublisher(ctx context.Context, input Actor, ready func(Evidence)) (Evidence, error) {
	observation := Evidence{Schema: observationSchema, Kind: "complete", Role: "publisher", PID: os.Getpid(),
		ManifestDigest: input.ManifestDigest, NetworkID: input.NetworkID, EpochDigest: input.EpochDigest,
		NodeID: input.NodeID, PreviousPin: input.UpstreamPin}
	if err := validatePublisher(input); err != nil {
		return observation, err
	}
	if input.MaximumAttachments > 1 {
		return servePublisherCapacity(ctx, input, ready, observation)
	}
	listener, err := net.Listen("tcp", input.ListenAddress)
	if err != nil {
		return observation, fmt.Errorf("listen for publisher: %w", err)
	}
	defer listener.Close()
	if err := bindListenerLifetime(ctx, listener.(*net.TCPListener), input.Role); err != nil {
		return observation, err
	}
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	if ready != nil {
		ready(Evidence{Schema: observationSchema, Kind: "ready", Role: "publisher", PID: os.Getpid(),
			NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, NodeID: input.NodeID, PreviousPin: input.UpstreamPin,
			DeadlineMillis: uint32(input.Deadline / time.Millisecond),
			LifetimeMillis: uint32(input.Lifetime / time.Millisecond)})
	}
	connection, err := listener.Accept()
	if err != nil {
		return observation, contextError(ctx, err)
	}
	stop()
	if err := listener.Close(); err != nil {
		connection.Close()
		return observation, fmt.Errorf("close publisher bounded Attachment listener: %w", err)
	}
	result, err := carryPublisherConnection(ctx, input, connection, observation, func() bool { return true })
	if err == nil {
		result.AttachmentsCompleted = 1
	}
	return result, err
}

func validatePublisher(input Actor) error {
	if !emptyPlan(input.Plan) || input.PublisherPin != [32]byte{} ||
		!emptyCertificate(input.ClientCertificate) || input.NextNodeID != [32]byte{} || input.NextAddress != "" ||
		input.NextPin != [32]byte{} || input.IntroductionSetupPublic != [32]byte{} ||
		input.IntroductionForwardSocket != "" || input.IntroductionForwardPublic != [32]byte{} ||
		input.IntroductionServicePublic != [32]byte{} {
		return errors.New("publisher received information outside its role-local duty")
	}
	if input.Role != "publisher" || input.MaximumAttachments > maximumRoleAttachments ||
		input.NetworkID == [32]byte{} || input.EpochDigest == [32]byte{} ||
		input.NodeID == [32]byte{} || input.UpstreamPin == [32]byte{} {
		return errors.New("publisher responder identity is required")
	}
	if err := validateCapacity(input); err != nil {
		return err
	}
	if err := validateEndpoint(input.ListenAddress); err != nil {
		return err
	}
	if err := validateCertificate(input.Certificate); err != nil {
		return err
	}
	setup := input.IntroductionSetupSocket != ""
	target := input.AttachmentTarget
	if target == 0 {
		target = max(input.MaximumAttachments, 1)
	}
	if target > 1 && (input.RawAttachment || input.Stream != nil || setup) {
		return errors.New("publisher multi-Attachment duty cannot share scoped stream or setup state")
	}
	if setup != (input.IntroductionSetupPeer != [32]byte{}) || setup != (input.IntroductionSetupNode != [32]byte{}) {
		return errors.New("publisher sealed setup service duty is incomplete")
	}
	if input.RawAttachment {
		if input.Stream == nil || !setup && !emptyCertificate(input.ServiceCertificate) {
			return errors.New("raw publisher attachment duty is invalid")
		}
	} else if err := validateCertificate(input.ServiceCertificate); err != nil {
		return err
	}
	if setup {
		if err := validateCertificate(input.ServiceCertificate); err != nil {
			return fmt.Errorf("validate sealed setup service certificate: %w", err)
		}
	}
	return validateDeadline(input.Deadline)
}
