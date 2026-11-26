package sourcegenesis

import (
	"errors"
	"io"
)

// Run executes the selected fixed Source genesis operation.
func Run(args []string, out io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "prepare-initial-plan":
			return runSourceInitialPlan(args[1:], out)
		case "prepare-initial-record", "prepare-initial-epoch", "sign-initial-record", "sign-initial-epoch":
			return runSourceInitialNetwork(args, out)
		case "prepare-initial-keys":
			return runSourceInitialKeys(args[1:], out)
		}
	}
	return errors.New("infrastructure provisioning operation unavailable")
}
