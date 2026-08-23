package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) == 0 || arguments[0] != "inspect-envelope" {
		return errors.New("usage: ardents-custody inspect-envelope [flags]")
	}
	flags := flag.NewFlagSet("inspect-envelope", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, path string
	flags.StringVar(&root, "vault-root", "", "exclusive custody vault root")
	flags.StringVar(&path, "envelope", "", "canonical custody envelope")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || root == "" || path == "" {
		return errors.New("inspect-envelope requires vault-root and envelope")
	}
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		return err
	}
	defer vault.Close()
	receipt, err := vault.Execute(ctx, custody.Operation{Kind: custody.OperationInspectEnvelope, Path: path}, nil)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema         string `json:"schema"`
		Operation      string `json:"operation"`
		Purpose        string `json:"purpose"`
		CiphertextSize uint64 `json:"ciphertext_size"`
		Digest         string `json:"digest"`
	}{
		Schema: "ardents-custody-inspection-v1", Operation: string(receipt.Operation), Purpose: string(receipt.Envelope.Purpose), CiphertextSize: receipt.Envelope.CiphertextSize, Digest: hex.EncodeToString(receipt.Envelope.Digest[:]),
	})
}
