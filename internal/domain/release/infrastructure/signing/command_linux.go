package signing

import (
	"errors"
	"io"
)

// Run selects one original offline Release operation for a supported role.
func Run(args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: ardents infrastructure sign-release ROLE OPERATION [ARGUMENTS]")
	}
	role, operation, operationArgs := nodeReleaseRole(args[0]), args[1], args[2:]
	if role == "source" {
		switch operation {
		case "prepare-release-keys":
			return runReleaseKeys(role, operationArgs, out)
		case "sign-initial-release":
			return runSourceReleaseSigning(operationArgs, out)
		case "sign-forward-release":
			return runSourceReleasePurpose(operationArgs, out, sourceReleaseForward)
		case "refresh-release-timestamp":
			return runSourceReleasePurpose(operationArgs, out, sourceReleaseTimestamp)
		}
	}
	if _, ok := role.artifacts(); ok {
		switch operation {
		case "prepare-release-keys":
			return runReleaseKeys(role, operationArgs, out)
		case "prepare-release-plan", "prepare-forward-release-plan":
			return runNodeReleasePlan(role, args[1:], out)
		case "sign-initial-release":
			return runNodeReleaseCommand(role, operationArgs, out, nodeReleaseInitial)
		case "sign-forward-release":
			return runNodeReleaseCommand(role, operationArgs, out, nodeReleaseForward)
		}
	}
	return errors.New("infrastructure Release operation unavailable")
}
