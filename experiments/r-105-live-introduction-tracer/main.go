//go:build ignore

// R-105 disposable live Introduction control tracer.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "failure="+err.Error())
		os.Exit(1)
	}
}

func dispatch(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("expected introduction, publisher, or user")
	}
	set := flag.NewFlagSet(arguments[0], flag.ContinueOnError)
	endpoint := set.String("endpoint", "", "Introduction loopback endpoint")
	deadlineUnix := set.Int64("deadline-unix", 0, "shared finite deadline")
	mode := set.String("mode", "exact", "synthetic experiment cell")
	if err := set.Parse(arguments[1:]); err != nil {
		return err
	}
	deadline := time.Unix(*deadlineUnix, 0).UTC()
	if *deadlineUnix == 0 || !time.Now().UTC().Before(deadline) || (*mode != "exact" && *mode != "replay" &&
		*mode != "header-tamper" && *mode != "ciphertext-tamper" && *mode != "withdrawn-slot") {
		return errors.New("experiment arguments are invalid")
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	result, err := runRole(ctx, arguments[0], *endpoint, deadline, *mode)
	if result.Schema != "" {
		if encodeErr := json.NewEncoder(os.Stdout).Encode(result); encodeErr != nil {
			return encodeErr
		}
	}
	return err
}
