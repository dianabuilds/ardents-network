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

// recoveryBundle executes the two participant-safe H4-1C Bundle operations.
// It intentionally has no activation or deletion route: Namespace owns the
// authenticated reconciliation witness, and destructive removal needs its own
// confirmed custody transition.
func recoveryBundle(ctx context.Context, mode string, arguments []string, output io.Writer, input custody.SecretInput) error {
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, record, bundle, environment, network, authorityRoot, kind, identity string
	flags.StringVar(&root, "vault-root", "", "exclusive custody vault root")
	flags.StringVar(&record, "record", "", "opaque vault record identifier")
	flags.StringVar(&bundle, "bundle", "", "recovery bundle path")
	flags.StringVar(&environment, "environment-commitment", "", "environment SHA-256 commitment")
	flags.StringVar(&network, "network-commitment", "", "network SHA-256 commitment")
	flags.StringVar(&authorityRoot, "root-commitment", "", "authority-root SHA-256 commitment")
	flags.StringVar(&kind, "kind", "", "authority kind: service or name")
	flags.StringVar(&identity, "id-commitment", "", "authority identity SHA-256 commitment")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || root == "" || bundle == "" || input == nil ||
		(mode == "export-recovery-bundle" && record == "") || (mode == "restore-recovery-bundle" && record != "") {
		return errors.New(mode + " requires vault-root, bundle, all public commitments, kind, and interactive secret input; export additionally requires record")
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
	operation := custody.Operation{RecordID: record, Expected: binding, Path: bundle}
	switch mode {
	case "export-recovery-bundle":
		operation.Kind = custody.OperationExportRecoveryBundle
	case "restore-recovery-bundle":
		operation.Kind = custody.OperationRestoreRecoveryBundle
	default:
		return errors.New("unsupported recovery bundle operation")
	}
	receipt, err := vault.Execute(ctx, operation, input)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema       string              `json:"schema"`
		Operation    string              `json:"operation"`
		RecordID     string              `json:"record_id"`
		State        custody.RecordState `json:"state"`
		TestRestored bool                `json:"test_restored"`
		Digest       string              `json:"digest"`
	}{Schema: "ardents-custody-recovery-bundle-v1", Operation: string(receipt.Operation), RecordID: receipt.RecordID,
		State: receipt.State, TestRestored: receipt.TestRestored, Digest: hex.EncodeToString(receipt.Envelope.Digest[:])})
}
