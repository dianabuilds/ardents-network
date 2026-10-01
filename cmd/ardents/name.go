package main

import (
	"errors"
	"io"
)

var errNameNetworkCommandRetired = errors.New("name network command is retired; protected Service Name access is not selected")

// runName refuses the whole retired name verb family before interpreting any
// remaining arguments or effects. The local canonical Stage 6 wire encoder
// died with its final consumer under ADR-0113; canonical Name syntax remains
// a possible future stage only under an entirely new scoped design.
func runName(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return nameUsageError()
	}
	switch arguments[0] {
	case "encode", "resolve", "control":
		return errNameNetworkCommandRetired
	default:
		return nameUsageError()
	}
}

func nameUsageError() error {
	return errors.New("usage: ardents name encode (retired) | resolve (retired) | control (retired)")
}
