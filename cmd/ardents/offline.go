package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) > 0 && arguments[0] == "endpoint" {
		return runEndpoint(ctx, arguments, output)
	}
	if len(arguments) > 0 && arguments[0] == "entry" {
		return runEntryImport(ctx, arguments, output)
	}
	if len(arguments) > 0 && arguments[0] == "name" {
		return runName(arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "refresh-sources" {
		return runRefreshSources(ctx, arguments, output)
	}
	if len(arguments) == 0 || arguments[0] != "accept-offline" {
		return errors.New("usage: ardents accept-offline [flags]")
	}
	flags := flag.NewFlagSet("accept-offline", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var raw rawConfig
	flags.StringVar(&raw.root, "state-root", "", "owned state root")
	flags.StringVar(&raw.network, "network-id", "", "32-byte network identity in hex")
	flags.StringVar(&raw.authorities, "authorities", "", "comma-separated Ed25519 public keys in hex")
	flags.IntVar(&raw.threshold, "threshold", 0, "signature threshold")
	flags.StringVar(&raw.at, "at", "", "verification time in RFC3339")
	flags.StringVar(&raw.epoch, "epoch", "", "canonical Epoch file")
	flags.StringVar(&raw.inputs, "inputs", "", "canonical input directory")
	flags.StringVar(&raw.material, "materialization", "", "canonical materialization file")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("accept-offline has unexpected positional arguments")
	}
	config, err := raw.networkStateConfig()
	if err != nil {
		return err
	}
	epoch, inputs, material, err := readOfflineInputs(raw)
	if err != nil {
		return err
	}
	store, err := state.Open(config)
	if err != nil {
		return fmt.Errorf("open network state: %w", err)
	}
	defer store.Close()
	snapshot, err := store.Accept(ctx, epoch, inputs, [][]byte{material})
	if err != nil {
		return fmt.Errorf("accept offline state: %w", err)
	}
	encoded := json.NewEncoder(output)
	encoded.SetEscapeHTML(false)
	return encoded.Encode(struct {
		Schema         string `json:"schema"`
		Sequence       uint64 `json:"sequence"`
		Kind           string `json:"kind"`
		Generation     string `json:"generation"`
		Epoch          uint64 `json:"epoch"`
		ViewLength     uint32 `json:"view_length"`
		RejectedLength uint32 `json:"rejected_length"`
	}{
		"ardents-h3-state-event-v1", 1, "generation-accepted",
		snapshot.Generation, snapshot.Epoch, snapshot.ViewLength, snapshot.RejectedLength,
	})
}
