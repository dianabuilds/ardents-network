package main

import "io"

type descriptorControlFailure struct{ cause error }

func (e *descriptorControlFailure) Error() string { return "Descriptor artifact preparation refused" }
func (e *descriptorControlFailure) Unwrap() error { return e.cause }

func runDescriptorControl(args []string, out io.Writer) error {
	if len(args) == 0 {
		return &descriptorControlFailure{errSourceControlInput}
	}
	switch args[0] {
	case "prepare-profile", "sign-profile":
		return runDescriptorProfileProvision(args, out)
	case "prepare-token-keys":
		return runDescriptorTokenMaterial(args[1:], out)
	case "prepare-state-key", "prepare-epoch", "sign-epoch":
		return runDescriptorEpochProvision(args, out)
	case "prepare-node-key", "prepare-node-record", "sign-node-record", "prepare-node-certificate":
		return runDescriptorNodeProvision(args, out)
	case "prepare-release-keys":
		return runDescriptorReleaseKeys(args[1:], out)
	case "sign-initial-release":
		return runNodeReleaseCommand(descriptorReleaseRole, args[1:], out, nodeReleaseInitial)
	case "sign-forward-release":
		return runNodeReleaseCommand(descriptorReleaseRole, args[1:], out, nodeReleaseForward)
	case "prepare-release-plan", "prepare-forward-release-plan":
	default:
		return &descriptorControlFailure{errSourceControlInput}
	}
	return runNodeReleasePlan(descriptorReleaseRole, args, out)
}
