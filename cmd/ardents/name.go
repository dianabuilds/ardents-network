package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

var errNameNetworkCommandRetired = errors.New("name network command is retired; protected Service Name access is not selected")

// runName adapts local canonical Name encoding and refuses the retired network
// commands before interpreting any of their remaining arguments.
func runName(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return nameUsageError()
	}
	switch arguments[0] {
	case "encode":
		if len(arguments) != 2 {
			return nameUsageError()
		}
		name, err := naming.Parse(arguments[1])
		if err != nil {
			return err
		}
		wire, err := naming.EncodeWire(name)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "%x\n", wire)
		return err
	case "resolve", "control":
		return errNameNetworkCommandRetired
	default:
		return nameUsageError()
	}
}

func nameUsageError() error {
	return errors.New("usage: ardents name encode <name> | resolve (retired) | control (retired)")
}
