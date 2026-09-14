package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "qualification worker unavailable")
		os.Exit(2)
	}
}

// run owns only the two static systemd worker entrypoints. It does not accept
// an address, Target, executable, profile, authority, Grant or environment
// supplied work shape; the installed Endpoint sends the fixed INIT record.
func run(arguments []string, input io.Reader, output io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("qualification worker command is invalid")
	}
	role := streamqualification.ReaderRole
	switch arguments[0] {
	case "worker-reader":
	case "worker-publisher":
		role = streamqualification.PublisherRole
	default:
		return errors.New("qualification worker command is invalid")
	}
	init, err := streamqualification.ReadInit(input, role)
	if err != nil {
		return err
	}
	return streamqualification.WriteReady(output, init.Nonce)
}
