package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	endpointunit "github.com/dianabuilds/ardents-network/internal/successor/installation/unit"
)

type managerProperties = systemd.Properties

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

// verifyPlatformRunningEndpointProperties admits the running platform-managed
// invocation. The F-25 ExecStartPre platform check transferred accepted
// current-program identity before this start; the binding below is read from the
// loaded pre-start argv and re-checked against the complete platform confinement
// and this exact running process. It grants no installation or start authority.
func verifyPlatformRunningEndpointProperties(version string, unit, service managerProperties, request installationRequest, generationDigest string, pid uint32, invocation [16]byte, checkProgram, manifest, pin string) error {
	expected, err := expectedUnit(request)
	if err != nil {
		return err
	}
	return unitObservationError(endpointunit.VerifyPlatformRunning(version, unit, service, expected, generationDigest, pid, invocation, checkProgram, manifest, pin))
}

// platformCheckBinding reads the sole privileged F-25 pre-start argv from the
// loaded ExecStartPreEx. The exact launch-declaration digest was verified by the
// platform check before start; this projection only retains that bound argv so
// the running confinement observation re-checks the same pre-start binding.
func platformCheckBinding(service managerProperties) (checkProgram, manifest, pin string, err error) {
	value, found := service["ExecStartPreEx"]
	var entries [][]json.RawMessage
	if !found || value.Type != "a(sasasttttuii)" || systemd.Decode(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return "", "", "", ErrBinding
	}
	var argv []string
	if systemd.Decode(entries[0][1], &argv) != nil || len(argv) != 5 || argv[1] != "platform" || argv[2] != "check" {
		return "", "", "", ErrBinding
	}
	var program string
	if systemd.Decode(entries[0][0], &program) != nil || program != argv[0] {
		return "", "", "", ErrBinding
	}
	return argv[0], argv[3], argv[4], nil
}

func verifyBoundActivationSocket(unit, socket managerProperties, role string) error {
	return unitObservationError(endpointunit.VerifyListeningSocket(unit, socket, role))
}
func verifyStoppedActivationSocket(unit, socket managerProperties, role string) error {
	return unitObservationError(endpointunit.VerifyStoppedSocket(unit, socket, role))
}

// Actual manager observations are checked against the fixed-unit contract.
// These predicates grant no Release, transition or start authority.

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

// Candidate cleanup separately verifies the typed Endpoint termination state.
// Initial/recovery admission still requires the inactive Endpoint above.
