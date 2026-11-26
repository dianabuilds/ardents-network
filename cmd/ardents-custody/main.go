package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/domain/custody/command"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	work := func() int {
		if err := command.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	}
	code := 2
	switch os.Getenv("ARDENTS_LOCAL_METRICS") {
	case "":
		code = work()
	case "1":
		code = measurement.Run(os.Stderr, work, nil)
	default:
		fmt.Fprintln(os.Stderr, "invalid local measurement configuration")
	}
	cancel()
	os.Exit(code)
}
