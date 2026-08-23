package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

// runName adapts the selected naming operator operations. Resolution and
// control retain their distinct authenticated input and receipt semantics.
func runName(arguments []string, output io.Writer) error {
	return runNameWithTransport(arguments, output, http.DefaultTransport.(*http.Transport))
}

func runNameWithTransport(arguments []string, output io.Writer, transport *http.Transport) error {
	return runNameWithRuntime(arguments, output, transport, currentResolution)
}

func runNameWithRuntime(arguments []string, output io.Writer, transport *http.Transport, load resolutionViewLoader) error {
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
	case "resolve":
		if len(arguments) != 4 {
			return nameUsageError()
		}
		isolation, err := decodeContext(arguments[3])
		if err != nil {
			return err
		}
		return runResolution(arguments[1], arguments[2], isolation, output, transport, load)
	case "control":
		if len(arguments) != 4 {
			return nameUsageError()
		}
		isolation, err := decodeContext(arguments[3])
		if err != nil {
			return err
		}
		return runControl(arguments[1], arguments[2], isolation, output, transport, load)
	default:
		return nameUsageError()
	}
}

func nameUsageError() error {
	return errors.New("usage: ardents name encode <name> | resolve <input-file> <name> <context-hex> | control <input-file> <operation-file> <context-hex>")
}
