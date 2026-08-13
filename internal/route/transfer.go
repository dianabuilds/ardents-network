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
		EpochDigest: input.Plan.Digest, Positions: append([]Position(nil), input.Plan.Positions...)}
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
	canary := append([]byte(nil), input.Canary...)
	if len(canary) == 0 {
		canary = make([]byte, canaryLength)
		if _, err := rand.Read(canary); err != nil {
			return evidence, fmt.Errorf("draw canary: %w", err)
		}
	}
	if len(canary) != canaryLength {
		return evidence, errors.New("canary must contain exactly 32 bytes")
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
