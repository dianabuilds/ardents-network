package main

import (
	"errors"
	"io"
)

type sourceControlFailure struct{ cause error }

func (e *sourceControlFailure) Error() string { return "source control failed" }
func (e *sourceControlFailure) Unwrap() error { return e.cause }

// The transitional Source install-input preparation is retired (ADR-0125, #604);
// the platform-led Source lifecycle replaces it. The refusal fires before any
// input is read or any effect is attempted.
var errSourceInstallInputsRetired = errors.New("old Source install-input preparation is retired; use the platform-led Source lifecycle (ardents-control source prepare-state, ardents-next platform initialize-trust)")

// The old Source unit template emitter is retired; the canonical platform-led
// template is produced by `ardents-next platform template`.
var errSourcePrepareTemplateRetired = errors.New("old Source prepare-template is retired; use ardents-next platform template")

// Each finite Source operation has its own purpose and input custody. No route
// constructs an Installation Startup or grants an installed runtime permission.
func runSourceControl(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: ardents-control source OPERATION [ARGUMENTS]")
	}
	switch args[0] {
	case "prepare-release-keys":
		return runSourceReleaseKeys(args[1:], out)
	case "prepare-initial-plan":
		return runSourceInitialPlan(args[1:], out)
	case "prepare-install-inputs":
		return errSourceInstallInputsRetired
	case "prepare-initial-record", "prepare-initial-epoch", "sign-initial-record", "sign-initial-epoch":
		return runSourceInitialNetwork(args, out)
	case "prepare-initial-keys":
		return runSourceInitialKeys(args[1:], out)
	case "sign-initial-release":
		return runSourceReleaseSigning(args[1:], out)
	case "sign-forward-release":
		return runSourceReleasePurpose(args[1:], out, sourceReleaseForward)
	case "refresh-release-timestamp":
		return runSourceReleasePurpose(args[1:], out, sourceReleaseTimestamp)
	case "prepare-template":
		return errSourcePrepareTemplateRetired
	case "prepare-descriptor", "prepare-inventory":
		return runSourceArtifactPreparation(args, out)
	case "inspect-latest":
		return runSourceInspection(args[1:], out)
	case "prepare-state":
		return runSourceStatePreparation(args[1:], out)
	default:
		return errors.New("usage: ardents-control source OPERATION [ARGUMENTS]")
	}
}
