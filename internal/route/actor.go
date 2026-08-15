package route

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"
)

// Run owns one bounded Client, Node-position, or Publisher process lifetime.
func Run(ctx context.Context, input Actor, ready func(Evidence)) (Evidence, error) {
	if input.ManifestDigest == [32]byte{} {
		return Evidence{}, errors.New("route manifest commitment is required")
	}
	if err := validateDeadline(input.Deadline); err != nil {
		return Evidence{}, err
	}
	lifetime := input.Lifetime
	if lifetime == 0 {
		lifetime = input.Deadline
	}
	if err := validateLifetime(input.Deadline, lifetime); err != nil {
		return Evidence{}, err
	}
	input.Lifetime = lifetime
	attempt, cancel := context.WithTimeout(ctx, lifetime)
	defer cancel()
	var acknowledgement <-chan error
	var introductionSetup <-chan introductionSetupResult
	var cleanups []func() error
	cleanup := func() error {
		var cleanupErr error
		for index := len(cleanups) - 1; index >= 0; index-- {
			cleanupErr = errors.Join(cleanupErr, cleanups[index]())
		}
		cleanups = nil
		return cleanupErr
	}
	if input.Role == "introduction" && input.AcknowledgementSocket != "" {
		stop, completed, err := startAcknowledgement(attempt, input.AcknowledgementSocket,
			input.AcknowledgementKeyFile)
		if err != nil {
			return Evidence{}, err
		}
		cleanups = append(cleanups, stop)
		acknowledgement = completed
	}
	if input.Role == "introduction" && input.IntroductionSetupSocket != "" {
		stop, completed, err := startIntroductionRelay(attempt, input)
		if err != nil {
			return Evidence{}, errors.Join(err, cleanup())
		}
		cleanups = append(cleanups, stop)
		introductionSetup = completed
	}
	if input.Role == "publisher" && input.IntroductionSetupSocket != "" {
		stop, completed, err := startIntroductionService(attempt, input)
		if err != nil {
			return Evidence{}, errors.Join(err, cleanup())
		}
		cleanups = append(cleanups, stop)
		introductionSetup = completed
	}
	var result Evidence
	var err error
	switch input.Role {
	case "client":
		if ready != nil {
			return Evidence{}, errors.New("client has no listener readiness event")
		}
		result, err = transfer(attempt, input)
	case "publisher":
		result, err = servePublisher(attempt, input, ready)
	case "initiator", "introduction", "rendezvous", "responder":
		result, err = serveNode(attempt, input, ready)
	default:
		return Evidence{}, errors.New("route actor role is invalid")
	}
	if acknowledgement != nil {
		err = errors.Join(err, <-acknowledgement)
	}
	if introductionSetup != nil {
		setup := <-introductionSetup
		result.IntroductionSetupReceipt = setup.receipt
		result.IntroductionSetup = setup.proof
		result.IntroductionOpaqueBytes = setup.opaqueBytes
		result.IntroductionOpaqueDigest = setup.opaqueDigest
		err = errors.Join(err, setup.err)
	}
	cleanupErr := cleanup()
	err = errors.Join(err, cleanupErr)
	result.ManifestDigest = input.ManifestDigest
	result.RuntimeID = runtimeIdentity()
	result.DeadlineMillis = uint32(input.Deadline / time.Millisecond)
	result.LifetimeMillis = uint32(lifetime / time.Millisecond)
	result.Cleanup = cleanupErr == nil
	result.Terminal = "success"
	if err != nil {
		result.Terminal = "error"
		result.Error = err.Error()
		result.Cancelled = attempt.Err() != nil || ctx.Err() != nil
	}
	result.SourceID, result.BuildDigest, err = buildIdentity(result.SourceID, result.BuildDigest, err)
	if err != nil && result.Terminal == "success" {
		result.Terminal = "error"
		result.Error = err.Error()
	}
	return result, err
}

func runtimeIdentity() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "host"
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return hostname
	}
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}

func emptyPlan(value Plan) bool {
	return value.NetworkID == [32]byte{} && value.Generation == "" && value.Epoch == 0 &&
		value.Digest == [32]byte{} && value.Profile == "" && value.ViewRoot == [32]byte{} && value.Seed == [32]byte{} && value.SelectionAt == 0 &&
		len(value.ExcludedIdentities) == 0 && len(value.ExcludedFamilies) == 0 && len(value.ExcludedDomains) == 0 &&
		len(value.Positions) == 0
}

func emptyCertificate(value tls.Certificate) bool {
	return len(value.Certificate) == 0 && value.PrivateKey == nil
}

func validateCapacity(input Actor) error {
	maximum, target := input.MaximumAttachments, input.AttachmentTarget
	if maximum == 0 {
		maximum = 1
	}
	if target == 0 {
		target = maximum
	}
	if maximum > maximumRoleAttachments || target > maximum ||
		input.ResourceProfile != "" && input.ResourceProfile != "h3-np1-v1" {
		return errors.New("route Attachment capacity contract is invalid")
	}
	return nil
}
