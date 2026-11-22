package main

import "io"

type forwardingControlFailure struct{ cause error }

func (e *forwardingControlFailure) Error() string { return "forwarding artifact preparation refused" }
func (e *forwardingControlFailure) Unwrap() error { return e.cause }

func runForwardingControl(args []string, out io.Writer) error {
	if len(args) == 0 {
		return &forwardingControlFailure{errSourceControlInput}
	}
	switch args[0] {
	case "prepare-profile", "sign-profile":
		return runForwardingProfileProvision(args, out)
	case "prepare-token-keys":
		return runForwardingTokenMaterial(args[1:], out)
	case "prepare-state-key", "prepare-epoch", "sign-epoch":
		return runForwardingEpochProvision(args, out)
	case "prepare-node-key", "prepare-node-record", "sign-node-record", "prepare-node-certificate":
		return runForwardingNodeProvision(args, out)
	case "prepare-release-keys":
		return runForwardingReleaseKeys(args[1:], out)
	case "sign-initial-release":
		return runNodeReleaseCommand(forwardingReleaseRole, args[1:], out, nodeReleaseInitial)
	case "sign-forward-release":
		return runNodeReleaseCommand(forwardingReleaseRole, args[1:], out, nodeReleaseForward)
	case "prepare-release-plan", "prepare-forward-release-plan":
	default:
		return &forwardingControlFailure{errSourceControlInput}
	}
	return runNodeReleasePlan(forwardingReleaseRole, args, out)
}
