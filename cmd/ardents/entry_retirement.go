package main

import (
	"context"
	"errors"
	"io"
)

var errEntryCommandRetired = errors.New("entry Invite command is retired; the closed Entry set has no operator import surface")

// runEntry refuses the retired Invite-root operator commands before
// interpreting any of their remaining arguments (ADR-0106). The former
// `entry import` and `entry recipient` verbs opened the older Invite root
// and its owner-local recipient identity; no working-tree code reads that
// root any longer. The closed Entry set consumed by the protected text
// Endpoint is owned by that Linux runtime and exposes no command surface.
func runEntry(_ context.Context, arguments []string, _ io.Writer) error {
	if len(arguments) < 2 {
		return entryUsageError()
	}
	switch arguments[1] {
	case "import", "recipient":
		return errEntryCommandRetired
	default:
		return entryUsageError()
	}
}

func entryUsageError() error {
	return errors.New("usage: ardents entry <import|recipient> (retired)")
}
