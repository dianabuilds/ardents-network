package provisioning

import (
	"errors"
	"io"
)

// Run selects one fixed offline role and performs one provisioning operation.
func Run(args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: provision-infrastructure ROLE OPERATION [ARGUMENTS]")
	}
	role, operation, operationArgs := nodeProvisioningRole(args[0]), args[1], args[2:]
	if role == "source" {
		switch operation {
		case "prepare-initial-plan":
			return runSourceInitialPlan(operationArgs, out)
		case "prepare-initial-record", "prepare-initial-epoch", "sign-initial-record", "sign-initial-epoch":
			return runSourceInitialNetwork(args[1:], out)
		case "prepare-initial-keys":
			return runSourceInitialKeys(operationArgs, out)
		}
	}
	if role.valid() {
		switch operation {
		case "prepare-node-key", "prepare-node-record", "sign-node-record", "prepare-node-certificate":
			return runNodeProvision(role, args[1:], out)
		case "prepare-state-key", "prepare-epoch", "sign-epoch":
			return runNodeEpochProvision(role, args[1:], out)
		case "prepare-token-keys":
			return runNodeTokenMaterial(role, operationArgs, out)
		case "prepare-profile", "sign-profile":
			return runNodeProfileProvision(role, args[1:], out)
		}
	}
	return errors.New("infrastructure provisioning operation unavailable")
}
