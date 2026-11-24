package command

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// ReadRolePlan owns the shared configuration flags and bounded file read.
// Role adapters decode their own declarations and open their original owners.
func ReadRolePlan(args []string) (raw []byte, retained bool, err error) {
	if len(args) != 2 {
		return nil, false, errors.New("invalid Node role arguments")
	}
	switch args[0] {
	case "--config":
	case "--retained-config":
		retained = true
	default:
		return nil, false, errors.New("invalid Node role configuration flag")
	}
	raw, err = artifacts.ReadBoundedFile(args[1], 16<<10)
	return raw, retained, err
}
