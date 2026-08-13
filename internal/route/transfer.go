package route

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"
)

func transfer(ctx context.Context, input Actor) (Evidence, error) {
	evidence := Evidence{Schema: observationSchema, Kind: "complete", Role: "client", PID: os.Getpid(),
		NetworkID: input.Plan.NetworkID, Generation: input.Plan.Generation, Epoch: input.Plan.Epoch,
		EpochDigest: input.Plan.Digest, Profile: input.Plan.Profile, ViewRoot: input.Plan.ViewRoot,
		SelectionSeed: input.Plan.Seed, SelectionAt: input.Plan.SelectionAt,
		ExcludedIdentities: append([][32]byte(nil), input.Plan.ExcludedIdentities...),
		ExcludedFamilies:   append([]string(nil), input.Plan.ExcludedFamilies...),
		ExcludedDomains:    append([]string(nil), input.Plan.ExcludedDomains...),
		Positions:          append([]Position(nil), input.Plan.Positions...)}
	if input.NetworkID != [32]byte{} || input.EpochDigest != [32]byte{} || input.NodeID != [32]byte{} ||
		input.ListenAddress != "" || !emptyCertificate(input.Certificate) || input.UpstreamPin != [32]byte{} ||
		input.NextNodeID != [32]byte{} || input.NextAddress != "" || input.NextPin != [32]byte{} ||
		!emptyCertificate(input.ServiceCertificate) {
		return evidence, errors.New("client received information outside its role-local duty")
	}
	if err := Validate(input.Plan); err != nil {
		return evidence, err
	}
	if err := validateCertificate(input.ClientCertificate); err != nil {
		return evidence, err
	}
	if input.PublisherPin == [32]byte{} {
		return evidence, errors.New("publisher test identity is required")
	}
	if err := validateDeadline(input.Deadline); err != nil {
		return evidence, err
	}
	canary := make([]byte, canaryLength)
	if _, err := rand.Read(canary); err != nil {
		return evidence, fmt.Errorf("draw canary: %w", err)
	}
	first := input.Plan.Positions[0]
	outer, err := dialTLS(ctx, first.Endpoint, input.ClientCertificate, first.PublicKey, input.Deadline)
	if err != nil {
		return evidence, fmt.Errorf("authenticate initiator: %w", err)
	}
	defer outer.Close()
	_ = outer.SetDeadline(time.Now().Add(input.Deadline))
	if err := confirmLegBinding(outer, input.Plan.NetworkID, input.Plan.Digest, first.NodeID); err != nil {
		return evidence, fmt.Errorf("confirm initiator Network State binding: %w", err)
	}
	inner := tls.Client(outer, clientTLS(tls.Certificate{}, input.PublisherPin))
	if err := inner.HandshakeContext(ctx); err != nil {
		return evidence, fmt.Errorf("authenticate publisher through Route: %w", err)
	}
	evidence.PeerAuthenticated = true
	if input.Stream != nil {
		defer input.Stream.Close()
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
