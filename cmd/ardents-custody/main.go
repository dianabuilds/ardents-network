package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	input := terminalSecretInput{terminal: os.Stdin, prompts: os.Stderr}
	if err := run(context.Background(), os.Args[1:], os.Stdout, input); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
