package main

import (
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/release/infrastructure/signing"
)

func main() {
	if err := signing.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
