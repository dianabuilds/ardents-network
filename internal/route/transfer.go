package route

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

func transfer(ctx context.Context, input Actor) (evidence Evidence, runErr error) {
	evidence = Evidence{Schema: observationSchema, Kind: "complete", Role: "client", PID: os.Getpid(),
		NetworkID: input.Plan.NetworkID, Generation: input.Plan.Generation, Epoch: input.Plan.Epoch,
		EpochDigest: input.Plan.Digest, Profile: input.Plan.Profile, ViewRoot: input.Plan.ViewRoot,
		SelectionSeed: input.Plan.Seed, SelectionAt: input.Plan.SelectionAt,
		ExcludedIdentities: append([][32]byte(nil), input.Plan.ExcludedIdentities...),
		ExcludedFamilies:   append([]string(nil), input.Plan.ExcludedFamilies...),
		ExcludedDomains:    append([]string(nil), input.Plan.ExcludedDomains...),
		Positions:          append([]Position(nil), input.Plan.Positions...)}
	if input.IntroductionSetupSocket != "" {
		setup, receipt, err := requestIntroductionSetup(ctx, input)
		if err != nil {
			return evidence, fmt.Errorf("perform sealed Introduction setup: %w", err)
		}
		evidence.IntroductionSetupReceipt = receipt
		evidence.IntroductionSetup = setup
	}
	canary := make([]byte, canaryLength)
	if _, err := rand.Read(canary); err != nil {
		return evidence, fmt.Errorf("draw canary: %w", err)
	}
	first := input.Plan.Positions[0]
	outer, closeEntry, err := openInitiator(ctx, input, first)
	if err != nil {
		return evidence, fmt.Errorf("authenticate initiator: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, outer.Close(), closeEntry()) }()
	_ = outer.SetDeadline(time.Now().Add(input.Deadline))
	if input.RawAttachment {
		if err := outer.SetDeadline(time.Time{}); err != nil {
			return evidence, fmt.Errorf("clear client raw Attachment setup deadline: %w", err)
		}
		evidence.PeerAuthenticated = true
		forward, reverse, streamErr := relayOpaque(input.Stream, outer)
		evidence.OpaqueBytes, evidence.OpaqueDigest = forward.count, forward.digest
		evidence.ReverseOpaqueBytes, evidence.ReverseOpaqueDigest = reverse.count, reverse.digest
		if streamErr != nil && !benignStreamError(streamErr) {
			return evidence, fmt.Errorf("carry raw Route Attachment: %w", streamErr)
		}
		return evidence, nil
	}
	inner := tls.Client(outer, clientTLS(tls.Certificate{}, input.PublisherPin))
	if err := inner.HandshakeContext(ctx); err != nil {
		return evidence, fmt.Errorf("authenticate publisher through Route: %w", err)
	}
	if err := inner.SetDeadline(time.Time{}); err != nil {
		return evidence, fmt.Errorf("clear client end-to-end setup deadline: %w", err)
	}
	evidence.PeerAuthenticated = true
	if input.Stream != nil {
		forward, reverse, streamErr := relayOpaque(input.Stream, inner)
		evidence.OpaqueBytes, evidence.OpaqueDigest = forward.count, forward.digest
		evidence.ReverseOpaqueBytes, evidence.ReverseOpaqueDigest = reverse.count, reverse.digest
		if streamErr != nil && !benignStreamError(streamErr) {
			return evidence, fmt.Errorf("carry bounded Service Connection stream: %w", streamErr)
		}
		return evidence, nil
	}
	if err := writeCanary(inner, canary); err != nil {
		return evidence, fmt.Errorf("write canary: %w", err)
	}
	result, err := readReceipt(inner, canary)
	if err != nil {
		return evidence, err
	}
	evidence.CanaryLength, evidence.CanaryDigest, evidence.Canary = result.length, result.digest, result.bytes
	return evidence, nil
}

func validateClient(input Actor) error {
	if input.NetworkID != [32]byte{} || input.EpochDigest != [32]byte{} || input.NodeID != [32]byte{} ||
		input.ListenAddress != "" || !emptyCertificate(input.Certificate) || input.UpstreamPin != [32]byte{} ||
		input.NextNodeID != [32]byte{} || input.NextAddress != "" || input.NextPin != [32]byte{} ||
		!emptyCertificate(input.ServiceCertificate) || input.IntroductionSetupPeer != [32]byte{} ||
		input.IntroductionForwardSocket != "" || input.IntroductionForwardPublic != [32]byte{} ||
		input.IntroductionSetupNode != [32]byte{} {
		return errors.New("client received information outside its role-local duty")
	}
	if err := Validate(input.Plan); err != nil {
		return err
	}
	if err := validateCertificate(input.ClientCertificate); err != nil {
		return err
	}
	if input.RawAttachment && input.Stream == nil {
		return errors.New("raw attachment stream is required")
	}
	if !input.RawAttachment && input.PublisherPin == [32]byte{} {
		return errors.New("publisher test identity is required")
	}
	if input.RawAttachment && input.PublisherPin != [32]byte{} {
		return errors.New("raw attachment exposes publisher identity to Route")
	}
	if (input.IntroductionSetupSocket == "") != (input.IntroductionSetupPublic == [32]byte{}) ||
		(input.IntroductionSetupSocket == "") != (input.IntroductionServicePublic == [32]byte{}) {
		return errors.New("sealed Introduction setup input is incomplete")
	}
	return validateDeadline(input.Deadline)
}

func openInitiator(ctx context.Context, input Actor, first Position) (*tls.Conn, func() error, error) {
	if input.OpenEntry == nil {
		connection, err := dialTLS(ctx, first.Endpoint, input.ClientCertificate, first.PublicKey, input.Deadline)
		if err != nil {
			return nil, func() error { return nil }, err
		}
		if err := confirmLegBinding(connection, input.Plan.NetworkID, input.Plan.Digest, first.NodeID); err != nil {
			return nil, func() error { return nil }, errors.Join(err, connection.Close())
		}
		return connection, func() error { return nil }, nil
	}
	authenticate := func(handshakeCtx context.Context, raw net.Conn) (*tls.Conn, error) {
		outer := tls.Client(raw, clientTLS(input.ClientCertificate, first.PublicKey))
		if err := outer.HandshakeContext(handshakeCtx); err != nil {
			return nil, err
		}
		if err := confirmLegBinding(outer, input.Plan.NetworkID, input.Plan.Digest, first.NodeID); err != nil {
			return nil, fmt.Errorf("confirm initiator Network State binding: %w", err)
		}
		return outer, nil
	}
	outer, cleanup, err := input.OpenEntry(ctx, authenticate)
	if err != nil {
		return nil, func() error { return nil }, err
	}
	if outer == nil || cleanup == nil {
		return nil, func() error { return nil }, errors.New("bridge entry opener returned incomplete ownership")
	}
	return outer, cleanup, nil
}
