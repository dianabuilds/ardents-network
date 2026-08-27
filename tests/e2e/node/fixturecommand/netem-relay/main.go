package main

import (
	"fmt"
	"os"
)

func main() {
	configuration, err := parseRelayConfiguration(os.Args[1:])
	if err == nil {
		err = runRelay(configuration)
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
