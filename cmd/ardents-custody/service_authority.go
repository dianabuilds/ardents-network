package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

func serviceAuthority(ctx context.Context, mode string, arguments []string, output io.Writer, input custody.SecretInput) error {
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, record, requestPath, responsePath, environment, network, authorityRoot, kind, identity string
	flags.StringVar(&root, "vault-root", "", "exclusive custody vault root")
	flags.StringVar(&record, "record", "", "current opaque Service Authority record identifier")
	flags.StringVar(&requestPath, "request", "", "canonical public Service Instance request")
	flags.StringVar(&responsePath, "response", "", "new canonical public Service Credential response")
	flags.StringVar(&environment, "environment-commitment", "", "environment SHA-256 commitment")
	flags.StringVar(&network, "network-commitment", "", "Network identifier")
	flags.StringVar(&authorityRoot, "root-commitment", "", "authority-root SHA-256 commitment")
	flags.StringVar(&kind, "kind", "", "fixed service authority kind")
	flags.StringVar(&identity, "id-commitment", "", "Service Authority identity commitment")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || root == "" || input == nil {
		return errors.New(mode + " requires a vault root, public inputs, and interactive secret input")
	}
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		return err
	}
	defer vault.Close()
	switch mode {
	case "create-service-authority":
		if record != "" || requestPath != "" || responsePath != "" || kind != "" || identity != "" {
			return errors.New("create-service-authority accepts no record, request, response, kind, or identity")
		}
		binding, err := newServiceBinding(environment, network, authorityRoot)
		if err != nil {
			return err
		}
		receipt, err := vault.Execute(ctx, custody.Operation{Kind: custody.OperationCreateServiceAuthority,
			Authority: custody.AuthorityState{Binding: binding}}, input)
		if err != nil {
			return err
		}
		return encodeServiceAuthorityReceipt(output, receipt)
	case "issue-service-credential":
		if record == "" || requestPath == "" || responsePath == "" {
			return errors.New("issue-service-credential requires record, request, and response")
		}
		binding, err := commandBinding(environment, network, authorityRoot, kind, identity)
		if err != nil || binding.Kind != custody.AuthorityService {
			return errors.New("issue-service-credential requires one exact Service Authority binding")
		}
		request, err := readPublicRequest(requestPath)
		if err != nil {
			return err
		}
		receipt, err := vault.Execute(ctx, custody.Operation{Kind: custody.OperationIssueServiceCredential,
			RecordID: record, Expected: binding, ServiceRequest: request}, input)
		if err != nil {
			return err
		}
		if err := writeStableCustodyPublicFile(responsePath, receipt.ServiceResponse); err != nil {
			return err
		}
		return encodeServiceCredentialReceipt(output, receipt)
	default:
		return errors.New("unsupported service Authority operation")
	}
}

func writeStableCustodyPublicFile(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := readPublicRequest(path)
		if readErr != nil || string(existing) != string(body) {
			return errors.New("service Credential response destination conflicts")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func newServiceBinding(environment, network, root string) (custody.AuthorityBinding, error) {
	var binding custody.AuthorityBinding
	for _, value := range []struct {
		text string
		dest []byte
	}{{environment, binding.Environment[:]}, {network, binding.Network[:]}, {root, binding.Root[:]}} {
		if err := decodeCommandCommitment(value.text, value.dest); err != nil {
			return custody.AuthorityBinding{}, errors.New("create-service-authority requires lowercase public commitments")
		}
	}
	binding.Kind = custody.AuthorityService
	return binding, nil
}

func readPublicRequest(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 1024 {
		return nil, errors.New("service Instance request file is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, 1025))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(raw) > 1024 {
		return nil, errors.Join(readErr, closeErr, errors.New("service Instance request file is invalid"))
	}
	return raw, nil
}

func encodeServiceAuthorityReceipt(output io.Writer, receipt custody.Receipt) error {
	return json.NewEncoder(output).Encode(struct {
		Schema          string `json:"schema"`
		RecordID        string `json:"record_id"`
		IDCommitment    string `json:"id_commitment"`
		AuthorityPublic string `json:"authority_public"`
		Target          string `json:"target"`
	}{Schema: "ardents-service-authority-v1", RecordID: receipt.RecordID,
		IDCommitment:    hex.EncodeToString(receipt.Authority.Binding.IDCommitment[:]),
		AuthorityPublic: hex.EncodeToString(receipt.ServiceAuthority.Public[:]),
		Target:          hex.EncodeToString(receipt.ServiceAuthority.Target[:])})
}

func encodeServiceCredentialReceipt(output io.Writer, receipt custody.Receipt) error {
	digest := sha256.Sum256(receipt.ServiceResponse)
	return json.NewEncoder(output).Encode(struct {
		Schema         string `json:"schema"`
		RecordID       string `json:"record_id"`
		Generation     uint64 `json:"generation"`
		Response       []byte `json:"response"`
		ResponseSHA256 string `json:"response_sha256"`
	}{Schema: "ardents-service-credential-response-v1", RecordID: receipt.RecordID,
		Generation: receipt.Authority.Watermarks[0].Value, Response: receipt.ServiceResponse,
		ResponseSHA256: hex.EncodeToString(digest[:])})
}
