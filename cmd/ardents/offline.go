package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func run(ctx context.Context, arguments []string, output io.Writer) (resultErr error) {
	if len(arguments) > 0 && arguments[0] == "endpoint" {
		return runEndpoint(ctx, arguments, output)
	}
	if len(arguments) > 0 && arguments[0] == "entry" {
		return runEntry(ctx, arguments, output)
	}
	if len(arguments) > 0 && arguments[0] == "name" {
		return runName(arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "service-instance" {
		return runServiceInstance(ctx, arguments[1:], output)
	}
	if len(arguments) > 0 && arguments[0] == "refresh-sources" {
		return runRefreshSources(ctx, arguments, output)
	}
	if len(arguments) > 0 && arguments[0] == "accept-closed-profile" {
		return runAcceptClosedProfile(arguments[1:], output)
	}
	if len(arguments) == 0 || arguments[0] != "accept-offline" {
		return errors.New("usage: ardents <accept-offline|accept-closed-profile|refresh-sources|endpoint|entry|name|service-instance> arguments")
	}
	return runAcceptOffline(ctx, arguments[1:], output)
}

func runAcceptOffline(ctx context.Context, arguments []string, output io.Writer) (resultErr error) {
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
	flags.StringVar(&raw.profile, "profile", "", "selected Network State profile")
	flags.StringVar(&raw.closedProfileAuthority, "closed-profile-authority", "", "pinned closed State profile signer")
	flags.StringVar(&raw.closedProfile, "closed-profile", "", "signed ARDCPR03 profile")
	if err := flags.Parse(arguments); err != nil {
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
	defer func() { resultErr = errors.Join(resultErr, store.Close()) }()
	snapshot, err := store.Accept(ctx, epoch, inputs, [][]byte{material})
	if err != nil {
		return fmt.Errorf("accept offline state: %w", err)
	}
	var profileDigest [32]byte
	profileDigestText := ""
	if raw.closedProfile != "" {
		profile, readErr := readOperatorInput(raw.closedProfile, 64<<10)
		if readErr != nil {
			return fmt.Errorf("read closed profile: %w", readErr)
		}
		view, acceptErr := store.AcceptClosedProfile(profile)
		if acceptErr != nil {
			return fmt.Errorf("accept closed profile: %w", acceptErr)
		}
		profileDigest = view.Digest
		profileDigestText = hex.EncodeToString(profileDigest[:])
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
		ClosedProfile  string `json:"closed_profile_sha256,omitempty"`
	}{
		"ardents-state-event-v1", 1, "generation-accepted",
		snapshot.Generation, snapshot.Epoch, snapshot.ViewLength, snapshot.RejectedLength, profileDigestText,
	})
}

// runAcceptClosedProfile submits one signed profile only to an already
// accepted State root. State owns all validation, idempotence, and durable
// conflict behavior; this command never accepts an Epoch or selects profile
// facts.
func runAcceptClosedProfile(arguments []string, output io.Writer) (resultErr error) {
	flags := flag.NewFlagSet("accept-closed-profile", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var raw rawConfig
	flags.StringVar(&raw.root, "state-root", "", "owned state root")
	flags.StringVar(&raw.network, "network-id", "", "32-byte network identity in hex")
	flags.StringVar(&raw.authorities, "authorities", "", "comma-separated Ed25519 public keys in hex")
	flags.IntVar(&raw.threshold, "threshold", 0, "signature threshold")
	flags.StringVar(&raw.at, "at", "", "verification time in RFC3339")
	flags.StringVar(&raw.profile, "profile", "", "selected Network State profile")
	flags.StringVar(&raw.closedProfileAuthority, "closed-profile-authority", "", "pinned closed State profile signer")
	flags.StringVar(&raw.closedProfile, "closed-profile", "", "signed ARDCPR03 profile")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("accept-closed-profile has unexpected positional arguments")
	}
	config, err := raw.networkStateConfig()
	if err != nil {
		return err
	}
	profile, err := readOperatorInput(raw.closedProfile, 64<<10)
	if err != nil {
		return fmt.Errorf("read closed profile: %w", err)
	}
	store, err := state.Open(config)
	if err != nil {
		return fmt.Errorf("open network state: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, store.Close()) }()
	view, err := store.AcceptClosedProfile(profile)
	if err != nil {
		return fmt.Errorf("accept closed profile: %w", err)
	}
	encoded := json.NewEncoder(output)
	encoded.SetEscapeHTML(false)
	return encoded.Encode(struct {
		Schema        string `json:"schema"`
		Sequence      uint64 `json:"sequence"`
		Kind          string `json:"kind"`
		Generation    string `json:"generation"`
		Epoch         uint64 `json:"epoch"`
		ClosedProfile string `json:"closed_profile_sha256"`
	}{
		Schema: "ardents-state-event-v1", Sequence: 1, Kind: "closed-profile-accepted",
		Generation: hex.EncodeToString(view.StateGeneration[:]), Epoch: view.Epoch,
		ClosedProfile: hex.EncodeToString(view.Digest[:]),
	})
}
