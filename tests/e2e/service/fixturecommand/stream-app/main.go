package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) > 0 && (arguments[0] == "direct-listen" || arguments[0] == "direct-connect") {
		return runDirectCommand(arguments, output)
	}
	return errors.New("usage: ardents-stream-app direct-<listen|connect> <address> <seed-file> <bytes>")
}
