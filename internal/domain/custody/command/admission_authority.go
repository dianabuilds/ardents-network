package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/custody"
)

type admissionRequestCommitmentInput interface {
	ReadAdmissionRequestCommitment(context.Context) ([32]byte, error)
}

func admissionAuthority(ctx context.Context, mode string, arguments []string, output io.Writer, input custody.SecretInput) error {
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, record, requestPath, permissionPath, environment, network, authorityRoot, kind, identity string
	flags.StringVar(&root, "vault-root", "", "exclusive custody vault root")
	flags.StringVar(&record, "record", "", "current opaque admission Authority record identifier")
	flags.StringVar(&requestPath, "request", "", "holder-signed admission allocation request")
	flags.StringVar(&permissionPath, "permission-output", "", "owner-only admission permission destination")
	flags.StringVar(&environment, "environment-commitment", "", "environment SHA-256 commitment")
	flags.StringVar(&network, "network-commitment", "", "Network identifier")
	flags.StringVar(&authorityRoot, "root-commitment", "", "closed authority-root SHA-256 commitment")
	flags.StringVar(&kind, "kind", "", "fixed admission authority kind")
	flags.StringVar(&identity, "id-commitment", "", "admission Authority identity commitment")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || root == "" || input == nil {
		return errors.New(mode + " requires a vault root, public inputs, and interactive secret input")
	}
	var operation custody.Operation
	switch mode {
	case "create-admission-authority":
		if record != "" || requestPath != "" || permissionPath != "" || kind != "" || identity != "" {
			return errors.New("create-admission-authority accepts no record, request, output, kind, or identity")
		}
		binding, err := newAuthorityBinding(mode, custody.AuthorityAdmission, environment, network, authorityRoot)
		if err != nil {
			return err
		}
		operation = custody.Operation{Kind: custody.OperationCreateAdmissionAuthority, Binding: binding}
	case "issue-admission-permission":
		if record == "" || requestPath == "" || permissionPath == "" {
			return errors.New("issue-admission-permission requires record, request, and permission-output")
		}
		binding, err := commandBinding(environment, network, authorityRoot, kind, identity)
		if err != nil || binding.Kind != custody.AuthorityAdmission {
			return errors.New("issue-admission-permission requires one exact admission Authority binding")
		}
		request, err := readAuthorityRequest(requestPath, 512, "admission permission request")
		if err != nil {
			return err
		}
		commitmentInput, ok := input.(admissionRequestCommitmentInput)
		if !ok {
			return errors.New("issue-admission-permission requires an independently transferred request commitment")
		}
		approved, err := commitmentInput.ReadAdmissionRequestCommitment(ctx)
		if err != nil {
			return err
		}
		if approved != sha256.Sum256(request) {
			return errors.New("admission request does not match the independently transferred commitment")
		}
		operation = custody.Operation{Kind: custody.OperationIssueAdmissionPermission, RecordID: record, Expected: binding,
			AdmissionRequest: request, AdmissionRequestCommitment: approved}
	default:
		return errors.New("unsupported admission Authority operation")
	}
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		return err
	}
	receipt, executeErr := vault.Execute(ctx, operation, input)
	if err := errors.Join(executeErr, vault.Close()); err != nil {
		return err
	}
	if mode == "create-admission-authority" {
		return json.NewEncoder(output).Encode(struct {
			Schema          string `json:"schema"`
			RecordID        string `json:"record_id"`
			IDCommitment    string `json:"id_commitment"`
			AuthorityPublic string `json:"authority_public"`
		}{Schema: "ardents-admission-authority-v1", RecordID: receipt.RecordID,
			IDCommitment:    hex.EncodeToString(receipt.Authority.Binding.IDCommitment[:]),
			AuthorityPublic: hex.EncodeToString(receipt.AdmissionAuthority.Public[:])})
	}
	if err := receipt.PublishAdmissionPermission(permissionPath); err != nil {
		return err
	}
	digest := sha256.Sum256(receipt.AdmissionPermission)
	return json.NewEncoder(output).Encode(struct {
		Schema           string `json:"schema"`
		RecordID         string `json:"record_id"`
		PermissionSHA256 string `json:"permission_sha256"`
	}{Schema: "ardents-admission-permission-receipt-v1", RecordID: receipt.RecordID,
		PermissionSHA256: hex.EncodeToString(digest[:])})
}
