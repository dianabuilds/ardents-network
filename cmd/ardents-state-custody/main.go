package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, newSecretInput()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
