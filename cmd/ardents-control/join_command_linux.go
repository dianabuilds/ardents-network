package main

import "io"

type joinControlFailure struct{ cause error }

func (e *joinControlFailure) Error() string { return "JOIN artifact preparation refused" }
func (e *joinControlFailure) Unwrap() error { return e.cause }

// JOIN has its own closed public producers. Shared native readers/writers own
// only physical input/output custody; no Source or Forwarding right is reused.
func runJoinControl(args []string, out io.Writer) error {
	if len(args) == 0 {
		return &joinControlFailure{errSourceControlInput}
	}
	switch args[0] {
	case "prepare-profile", "sign-profile":
		return runJoinProfileProvision(args, out)
	case "prepare-token-keys":
		return runJoinTokenMaterial(args[1:], out)
	case "prepare-state-key", "prepare-epoch", "sign-epoch":
		return runJoinEpochProvision(args, out)
	case "prepare-node-key", "prepare-node-record", "sign-node-record", "prepare-node-certificate":
		return runJoinNodeProvision(args, out)
	case "prepare-release-keys":
		return runJoinReleaseKeys(args[1:], out)
	case "sign-initial-release":
		return runNodeReleaseCommand(joinReleaseRole, args[1:], out, nodeReleaseInitial)
	case "sign-forward-release":
		return runNodeReleaseCommand(joinReleaseRole, args[1:], out, nodeReleaseForward)
	case "prepare-release-plan", "prepare-forward-release-plan":
	default:
		return &joinControlFailure{errSourceControlInput}
	}
	return runNodeReleasePlan(joinReleaseRole, args, out)
}
