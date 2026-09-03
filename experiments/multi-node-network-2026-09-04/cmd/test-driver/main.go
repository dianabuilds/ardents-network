//go:build ignore

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// main is the test-driver entrypoint. It installs the SIGINT/SIGTERM
// context, dispatches to the run() switch on os.Args[1], and exits 2 on
// any error. The subcommand implementations live in prebake.go (prebake,
// prebake_adversary), verify.go (verify, verify_adversary), and selftest.go
// (self-test). The five subcommands cover both the slice 1 (alpha_round_trip)
// and slice 2 (adversary_rejected) pilot scenarios.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "test-driver:", err)
		os.Exit(2)
	}
}

// run dispatches the five test-driver subcommands. The usage string is the
// single source of truth for what the binary accepts and must be kept in
// sync with the slice 1 and slice 2 docker-compose service commands.
func run(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: test-driver <prebake|prebake_adversary|verify|verify_adversary|self-test> EVIDENCE_DIR")
	}
	switch arguments[0] {
	case "prebake":
		return runPrebake(ctx, arguments[1:], false)
	case "prebake_adversary":
		return runPrebake(ctx, arguments[1:], true)
	case "verify":
		return runVerify(ctx, arguments[1:])
	case "verify_adversary":
		return runVerifyAdversary(ctx, arguments[1:])
	case "self-test":
		return runSelfTest(ctx)
	default:
		return fmt.Errorf("unknown subcommand %q", arguments[0])
	}
}
