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

func verifyRecord(ctx context.Context, arguments []string, output io.Writer, input custody.SecretInput) error {
	flags := flag.NewFlagSet("verify-record", flag.ContinueOnError)
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
		return errors.New("verify-record requires vault-root, record, all public commitments, kind, and interactive secret input")
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
	receipt, err := vault.Execute(ctx, custody.Operation{Kind: custody.OperationVerifyVaultRecord, RecordID: record, Expected: binding}, input)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema     string              `json:"schema"`
		Operation  string              `json:"operation"`
		RecordID   string              `json:"record_id"`
		State      custody.RecordState `json:"state"`
		Generation uint64              `json:"generation"`
		Revision   uint64              `json:"revision"`
		Watermarks []custody.Watermark `json:"watermarks"`
	}{Schema: "ardents-custody-verification-v1", Operation: string(receipt.Operation), RecordID: receipt.RecordID, State: receipt.State, Generation: receipt.Authority.Generation, Revision: receipt.Authority.Revision, Watermarks: receipt.Authority.Watermarks})
}

func commandBinding(environment, network, root, kind, identity string) (custody.AuthorityBinding, error) {
	var binding custody.AuthorityBinding
	for _, value := range []struct {
		text string
		dest []byte
	}{
		{environment, binding.Environment[:]},
		{network, binding.Network[:]},
		{root, binding.Root[:]},
		{identity, binding.IDCommitment[:]},
	} {
		if err := decodeCommandCommitment(value.text, value.dest); err != nil {
			return custody.AuthorityBinding{}, errors.New("verify-record requires lowercase SHA-256 commitments")
		}
	}
	binding.Kind = custody.AuthorityKind(kind)
	if binding.Kind != custody.AuthorityService && binding.Kind != custody.AuthorityName {
		return custody.AuthorityBinding{}, errors.New("verify-record kind must be service or name")
	}
	return binding, nil
}

func decodeCommandCommitment(value string, destination []byte) error {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(value) != 64 || len(decoded) != len(destination) || hex.EncodeToString(decoded) != value {
		return errors.New("invalid lowercase SHA-256 commitment")
	}
	copy(destination, decoded)
	return nil
}
