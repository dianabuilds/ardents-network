package main

import (
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

func main() {
	if err := provisioning.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
