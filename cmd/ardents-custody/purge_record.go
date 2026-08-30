package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

// purgeRecord is the separately confirmed destructive Custody action. It can
// remove one exact encrypted record but neither changes Authority floors nor
// touches a Recovery Bundle destination, program, or Endpoint state.
func purgeRecord(ctx context.Context, arguments []string, output io.Writer, input custody.SecretInput) error {
	flags := flag.NewFlagSet("purge-record", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, record, environment, network, authorityRoot, kind, identity string
	flags.StringVar(&root, "vault-root", "", "exclusive custody vault root")
	flags.StringVar(&record, "record", "", "opaque vault record identifier")
	flags.StringVar(&environment, "environment-commitment", "", "environment SHA-256 commitment")
	flags.StringVar(&network, "network-commitment", "", "network SHA-256 commitment")
	flags.StringVar(&authorityRoot, "root-commitment", "", "authority-root SHA-256 commitment")
	flags.StringVar(&kind, "kind", "", "authority kind: service or name")
	flags.StringVar(&identity, "id-commitment", "", "authority identity SHA-256 commitment")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || root == "" || record == "" || input == nil {
		return errors.New("purge-record requires vault-root, record, all public commitments, kind, and interactive secret confirmation")
	}
	binding, err := commandBinding(environment, network, authorityRoot, kind, identity)
	if err != nil {
		return err
	}
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		return err
	}
	defer vault.Close()
	receipt, err := vault.Execute(ctx, custody.Operation{Kind: custody.OperationPurgeVaultRecord, RecordID: record, Expected: binding}, input)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema, Operation, RecordID string
		RetainedFloor               bool `json:"retained_floor"`
	}{Schema: "ardents-custody-purge-v1", Operation: string(receipt.Operation), RecordID: receipt.RecordID, RetainedFloor: true})
}
