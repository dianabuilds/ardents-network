package installation

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	endpointunit "github.com/dianabuilds/ardents-network/internal/successor/installation/unit"
	"strings"
)

type managerProperties = systemd.Properties

// Request write-root admission stays with Installation. The fixed unit Module
// receives detached expected facts, never the private request or live custody.
func expectedUnit(request installationRequest) (endpointunit.Configuration, error) {
	paths, err := writableDirectories(request)
	return endpointunit.Configuration{InstallationRoot: request.InstallationRoot, WritePaths: paths}, err
}

func unitObservationError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, endpointunit.ErrInput):
		return errors.Join(ErrInput, err)
	case errors.Is(err, endpointunit.ErrNativeUnavailable):
		return errors.Join(ErrNativeUnavailable, err)
	default:
		return errors.Join(ErrBinding, err)
	}
}

func verifyStoppedEndpointProperties(version string, unit, service managerProperties, request installationRequest, generationDigest string) error {
	expected, err := expectedUnit(request)
	if err != nil {
		return err
	}
	return unitObservationError(endpointunit.VerifyFreshStopped(version, unit, service, expected, generationDigest))
}

func verifyEndpointConfiguration(version string, unit, service managerProperties, request installationRequest, generationDigest string) error {
	expected, err := expectedUnit(request)
	if err != nil {
		return err
	}
	return unitObservationError(endpointunit.VerifyConfiguration(version, unit, service, expected, generationDigest))
}

func verifyRunningEndpointProperties(version string, unit, service managerProperties, request installationRequest, generationDigest string, pid uint32, invocation [16]byte) error {
	expected, err := expectedUnit(request)
	if err != nil {
		return err
	}
	return unitObservationError(endpointunit.VerifyRunning(version, unit, service, expected, generationDigest, pid, invocation))
}

func verifyQuiescentEndpointProperties(version string, unit, service managerProperties, request installationRequest, generationDigest string) error {
	expected, err := expectedUnit(request)
	if err != nil {
		return err
	}
	return unitObservationError(endpointunit.VerifyQuiescent(version, unit, service, expected, generationDigest))
}

func endpointManagerInvocation(unit, service managerProperties) (uint32, [16]byte, error) {
	pid, invocation, err := endpointunit.Invocation(unit, service)
	return pid, invocation, unitObservationError(err)
}
func verifyNoQueuedEndpointJob(unit managerProperties) error {
	return unitObservationError(endpointunit.VerifyNoJob(unit))
}
func verifyStoppedAttemptProperties(version string, unit, service managerProperties, request installationRequest, generationDigest string) error {
	expected, err := expectedUnit(request)
	if err != nil {
		return err
	}
	return unitObservationError(endpointunit.VerifyStoppedAttempt(version, unit, service, expected, generationDigest))
}
func verifyBoundActivationSocket(unit, socket managerProperties, role string) error {
	return unitObservationError(endpointunit.VerifyListeningSocket(unit, socket, role))
}
func verifyStoppedActivationSocket(unit, socket managerProperties, role string) error {
	return unitObservationError(endpointunit.VerifyStoppedSocket(unit, socket, role))
}
func verifyStoppedManagerUnit(body, name string) error {
	return unitObservationError(endpointunit.VerifyStoppedUnit(body, name))
}

// Actual manager observations are checked against the fixed-unit contract.
// These predicates grant no Release, transition or start authority.
func observeBoundActivationSockets(ctx context.Context) error {
	return observeActivationSockets(ctx, false)
}

func observeStoppedActivationSockets(ctx context.Context) error {
	return observeActivationSockets(ctx, true)
}

func observeActivationSockets(ctx context.Context, stopped bool) error {
	if ctx == nil {
		return ErrInput
	}
	for _, role := range []string{"reader", "publisher"} {
		unit, socket, err := systemd.Activation(ctx, role)
		if err != nil {
			return err
		}
		if stopped {
			err = verifyStoppedActivationSocket(unit, socket, role)
		} else {
			err = verifyBoundActivationSocket(unit, socket, role)
		}
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}

func observeStoppedManager(ctx context.Context) error {
	body, err := systemd.State(ctx, "ardents-endpoint.service")
	if err != nil {
		return errors.Join(ErrNativeUnavailable, err)
	}
	if err := verifyStoppedManagerUnit(body, "ardents-endpoint.service"); err != nil {
		return err
	}
	return observeStoppedActivationManager(ctx)
}

// Candidate cleanup separately verifies the typed Endpoint termination state.
// Initial/recovery admission still requires the inactive Endpoint above.
func observeStoppedActivationManager(ctx context.Context) error {
	for _, unit := range []string{"ardents-text-reader.socket", "ardents-text-publisher.socket"} {
		body, err := systemd.State(ctx, unit)
		if err != nil {
			return errors.Join(ErrNativeUnavailable, err)
		}
		if err := verifyStoppedManagerUnit(body, unit); err != nil {
			return err
		}
	}
	body, err := systemd.Instances(ctx)
	if err != nil || strings.TrimSpace(body) != "" {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return ctx.Err()
}
