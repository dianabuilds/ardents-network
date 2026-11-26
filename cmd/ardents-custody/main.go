package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/dianabuilds/ardents-network/internal/domain/custody/command"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := command.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
