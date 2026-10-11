package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	input := terminalSecretInput{terminal: os.Stdin, prompts: os.Stderr}
	if err := run(ctx, os.Args[1:], os.Stdout, input); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
