package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/release/custody"
)

func run(ctx context.Context, arguments []string, output io.Writer, input custody.SecretInput) error {
	if len(arguments) == 0 || (arguments[0] != "initialize" && arguments[0] != "inspect") {
		return errors.New("usage: ardents-release-custody <initialize|inspect> --root ABSOLUTE_OWNER_ONLY_DIRECTORY")
	}
	operation := arguments[0]
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root string
	flags.StringVar(&root, "root", "", "owner-only release custody directory")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 || !filepath.IsAbs(root) {
		return errors.New("release custody initialization arguments are invalid")
	}
	var (
		receipt custody.Receipt
		err     error
	)
	if operation == "initialize" {
		receipt, err = custody.Initialize(ctx, custody.InitializeConfig{Root: root}, input)
	} else {
		receipt, err = custody.Inspect(ctx, custody.InspectConfig{Root: root}, input)
	}
	if err != nil {
		return err
	}
	roles := make([]struct {
		Role   string `json:"role"`
		Public string `json:"public"`
	}, len(receipt.Roles))
	for index, role := range receipt.Roles {
		roles[index].Role = role.Role
		roles[index].Public = hex.EncodeToString(role.Public[:])
	}
	return json.NewEncoder(output).Encode(struct {
		Schema   string `json:"schema"`
		Envelope string `json:"envelope_sha256"`
		Roles    any    `json:"roles"`
	}{Schema: "ardents-release-custody-receipt-v1", Envelope: hex.EncodeToString(receipt.EnvelopeDigest[:]), Roles: roles})
}
