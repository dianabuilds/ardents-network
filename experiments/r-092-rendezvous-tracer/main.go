//go:build ignore

// R-092 disposable Rendezvous reservation and data-plane tracer.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type synchronizedEncoder struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func (output *synchronizedEncoder) encode(value any) {
	output.mu.Lock()
	defer output.mu.Unlock()
	_ = output.encoder.Encode(value)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	output := &synchronizedEncoder{encoder: json.NewEncoder(os.Stdout)}
	if err := dispatch(ctx, os.Args[1:], output); err != nil {
		fmt.Fprintln(os.Stderr, "failure="+err.Error())
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, arguments []string, output *synchronizedEncoder) error {
	if len(arguments) == 0 {
		return errors.New("expected server or client subcommand")
	}
	switch arguments[0] {
	case "server":
		parsed, err := parseServerArguments(arguments[1:])
		if err != nil {
			return err
		}
		result, runErr := runServer(ctx, parsed, output.encode)
		if result.Schema != "" {
			output.encode(result)
		}
		return runErr
	case "client":
		parsed, err := parseClientArguments(arguments[1:])
		if err != nil {
			return err
		}
		clientCtx, cancel := context.WithDeadline(ctx, parsed.deadline)
		defer cancel()
		result, runErr := runClient(clientCtx, parsed)
		if result.Schema != "" {
			output.encode(result)
		}
		return runErr
	default:
		return errors.New("unknown Rendezvous tracer subcommand")
	}
}

func parseServerArguments(arguments []string) (serverArguments, error) {
	set := flag.NewFlagSet("server", flag.ContinueOnError)
	listen := set.String("listen", "", "literal TCP listen address")
	deadlineUnix := set.Int64("deadline-unix", 0, "shared binding deadline")
	handshakes := set.Int("handshakes", 0, "maximum concurrent handshakes")
	waiting := set.Int("waiting", 0, "maximum authenticated waiting legs")
	pairs := set.Int("pairs", 0, "maximum active pairs")
	expectPairs := set.Int("expect-pairs", 0, "successful pairs required before drain")
	drainAfterMS := set.Int("drain-after-ms", 0, "force an owned drain after this duration")
	if err := set.Parse(arguments); err != nil {
		return serverArguments{}, err
	}
	deadline := time.Unix(*deadlineUnix, 0).UTC()
	if *listen == "" || !deadline.After(time.Now().UTC()) || *handshakes < 1 || *handshakes > 64 ||
		*waiting < 1 || *waiting > 128 || *pairs < 1 || *pairs > 32 || *expectPairs < 0 || *expectPairs > 32 {
		return serverArguments{}, errors.New("server arguments are incomplete or outside experiment bounds")
	}
	if *drainAfterMS < 0 || *drainAfterMS > 30_000 {
		return serverArguments{}, errors.New("server drain bound is outside experiment bounds")
	}
	return serverArguments{listen: *listen, deadline: deadline, handshakeLimit: *handshakes,
		waitingLimit: *waiting, pairLimit: *pairs, expectPairs: *expectPairs,
		drainAfter: time.Duration(*drainAfterMS) * time.Millisecond}, nil
}

func parseClientArguments(arguments []string) (clientArguments, error) {
	set := flag.NewFlagSet("client", flag.ContinueOnError)
	endpoint := set.String("endpoint", "", "literal Rendezvous endpoint")
	side := set.String("side", "", "initiator or responder")
	token := set.String("token", "", "synthetic attachment token")
	deadlineUnix := set.Int64("deadline-unix", 0, "shared binding deadline")
	holdMS := set.Int("hold-ms", 0, "hold after authenticated binding")
	if err := set.Parse(arguments); err != nil {
		return clientArguments{}, err
	}
	deadline := time.Unix(*deadlineUnix, 0).UTC()
	if *endpoint == "" || (*side != "initiator" && *side != "responder") || *token == "" ||
		!deadline.After(time.Now().UTC()) || *holdMS < 0 || *holdMS > 30_000 {
		return clientArguments{}, errors.New("client arguments are incomplete or outside experiment bounds")
	}
	return clientArguments{endpoint: *endpoint, side: *side, token: *token, deadline: deadline,
		hold: time.Duration(*holdMS) * time.Millisecond}, nil
}
