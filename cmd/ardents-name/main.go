package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/naming"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(arguments []string, output io.Writer) error {
	return runWithTransport(arguments, output, http.DefaultTransport.(*http.Transport))
}

func runWithTransport(arguments []string, output io.Writer, transport *http.Transport) error {
	return runWithRuntime(arguments, output, transport, currentSnapshot)
}

func runWithRuntime(arguments []string, output io.Writer, transport *http.Transport, load snapshotLoader) error {
	if len(arguments) == 0 {
		return usageError()
	}
	switch arguments[0] {
	case "encode-name":
		if len(arguments) != 2 {
			return usageError()
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
	case "validate-record":
		if len(arguments) != 2 {
			return usageError()
		}
		wire, err := readBoundedRecord(arguments[1])
		if err != nil {
			return err
		}
		record, err := namelease.DecodeRecord(wire)
		if err != nil {
			return err
		}
		_, err = namelease.EncodeRecord(record)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, "valid")
		return err
	case "resolve":
		if len(arguments) != 4 {
			return usageError()
		}
		isolation, err := decodeContext(arguments[3])
		if err != nil {
			return err
		}
		return runResolution(arguments[1], arguments[2], isolation, output, transport, load)
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: ardents-name encode-name <name> | validate-record <file> | resolve <input-file> <name> <context-hex>")
}
