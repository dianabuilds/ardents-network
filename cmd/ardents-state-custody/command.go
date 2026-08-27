package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type alphaGenesisInitializer func(context.Context, state.AlphaGenesisConfig, state.AlphaGenesisSecretInput) (state.AlphaGenesisReceipt, error)

func run(ctx context.Context, arguments []string, output io.Writer, input state.AlphaGenesisSecretInput) error {
	return runWithInitializer(ctx, arguments, output, input, state.InitializeAlphaGenesis)
}

func runWithInitializer(ctx context.Context, arguments []string, output io.Writer, input state.AlphaGenesisSecretInput,
	initialize alphaGenesisInitializer) error {
	if len(arguments) == 0 || arguments[0] != "initialize-alpha-genesis" {
		return errors.New("usage: ardents-state-custody initialize-alpha-genesis --root <absolute-owner-directory>")
	}
	flags := flag.NewFlagSet(arguments[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root string
	flags.StringVar(&root, "root", "", "owner-only State custody directory")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 || !filepath.IsAbs(root) {
		return errors.New("functional alpha State custody arguments are invalid")
	}
	receipt, err := initialize(ctx, state.AlphaGenesisConfig{Root: root}, input)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema          string   `json:"schema"`
		EnvelopeSHA256  string   `json:"envelope_sha256"`
		NetworkID       string   `json:"network_id"`
		AuthorityPublic string   `json:"authority_public"`
		EpochDigest     string   `json:"epoch_digest"`
		Profile         string   `json:"profile"`
		Threshold       uint8    `json:"threshold"`
		NotBefore       string   `json:"not_before"`
		NotAfter        string   `json:"not_after"`
		Epoch           []byte   `json:"epoch"`
		Inputs          [][]byte `json:"inputs"`
		Materials       [][]byte `json:"materials"`
	}{
		Schema:         "ardents-functional-alpha-state-receipt-v1",
		EnvelopeSHA256: hex.EncodeToString(receipt.EnvelopeDigest[:]), NetworkID: hex.EncodeToString(receipt.NetworkID[:]),
		AuthorityPublic: hex.EncodeToString(receipt.AuthorityPublic[:]), EpochDigest: hex.EncodeToString(receipt.EpochDigest[:]),
		Profile: receipt.Profile, Threshold: receipt.Threshold, NotBefore: receipt.NotBefore.Format("2006-01-02T15:04:05Z"),
		NotAfter: receipt.NotAfter.Format("2006-01-02T15:04:05Z"), Epoch: append([]byte(nil), receipt.Epoch...),
		Inputs: make([][]byte, 0), Materials: make([][]byte, 0),
	})
}
