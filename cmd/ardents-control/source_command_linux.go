package main

import (
	"errors"
	"io"
)

type sourceControlFailure struct{ cause error }

func (e *sourceControlFailure) Error() string { return "source control failed" }
func (e *sourceControlFailure) Unwrap() error { return e.cause }

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
		return runSourceInstallInputs(args[1:], out)
	case "prepare-initial-record", "prepare-initial-epoch", "sign-initial-record", "sign-initial-epoch":
		return runSourceInitialNetwork(args, out)
	case "prepare-initial-keys":
		return runSourceInitialKeys(args[1:], out)
	case "sign-initial-release":
		return runSourceReleaseSigning(args[1:], out)
	case "sign-forward-release":
		return runSourceReleasePurpose(args[1:], out, true)
	case "prepare-template", "prepare-descriptor", "prepare-inventory":
		return runSourceArtifactPreparation(args, out)
	case "inspect-latest":
		return runSourceInspection(args[1:], out)
	case "prepare-state":
		return runSourceStatePreparation(args[1:], out)
	default:
		return errors.New("usage: ardents-control source OPERATION [ARGUMENTS]")
	}
}
