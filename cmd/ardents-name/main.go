package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/naming"
)

const maxRecordInput = 16 << 20

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) != 2 {
		return errors.New("usage: ardents-name encode-name <name> | validate-record <file>")
	}
	switch arguments[0] {
	case "encode-name":
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
	default:
		return errors.New("unknown ardents-name action")
	}
}

func readBoundedRecord(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	wire, err := io.ReadAll(io.LimitReader(file, maxRecordInput+1))
	if err != nil {
		return nil, err
	}
	if len(wire) == 0 || len(wire) > maxRecordInput {
		return nil, errors.New("name record input is empty or exceeds command bound")
	}
	return wire, nil
}
