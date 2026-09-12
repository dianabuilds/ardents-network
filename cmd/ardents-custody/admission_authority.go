package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

const maximumAdmissionPermissionBytes = 228

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
		binding, err := newAdmissionBinding(environment, network, authorityRoot)
		if err != nil {
			return err
		}
		operation = custody.Operation{Kind: custody.OperationCreateAdmissionAuthority, Authority: custody.AuthorityState{Binding: binding}}
	case "issue-admission-permission":
		if record == "" || requestPath == "" || permissionPath == "" {
			return errors.New("issue-admission-permission requires record, request, and permission-output")
		}
		binding, err := commandBinding(environment, network, authorityRoot, kind, identity)
		if err != nil || binding.Kind != custody.AuthorityAdmission {
			return errors.New("issue-admission-permission requires one exact admission Authority binding")
		}
		request, err := readAdmissionRequest(requestPath)
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
	if err := publishStableCustodyPrivateFile(permissionPath, receipt.AdmissionPermission); err != nil {
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

func newAdmissionBinding(environment, network, root string) (custody.AuthorityBinding, error) {
	var binding custody.AuthorityBinding
	for _, value := range []struct {
		text string
		dest []byte
	}{{environment, binding.Environment[:]}, {network, binding.Network[:]}, {root, binding.Root[:]}} {
		if err := decodeCommandCommitment(value.text, value.dest); err != nil {
			return custody.AuthorityBinding{}, errors.New("create-admission-authority requires lowercase public commitments")
		}
	}
	binding.Kind = custody.AuthorityAdmission
	return binding, nil
}

func readAdmissionRequest(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 512 {
		return nil, errors.New("admission permission request file is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, 513))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(raw) > 512 {
		return nil, errors.Join(readErr, closeErr, errors.New("admission permission request file is invalid"))
	}
	return raw, nil
}

func publishStableCustodyPrivateFile(path string, body []byte) (resultErr error) {
	if len(body) != maximumAdmissionPermissionBytes {
		return errors.New("admission permission is invalid")
	}
	destination, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve admission permission destination: %w", err)
	}
	parent := filepath.Dir(destination)
	staging, err := os.CreateTemp(parent, ".ardents-admission-permission-")
	if err != nil {
		return fmt.Errorf("create admission permission staging file: %w", err)
	}
	stagingPath, stagingOpen := staging.Name(), true
	defer func() {
		if stagingOpen {
			resultErr = errors.Join(resultErr, staging.Close())
		}
		if removeErr := os.Remove(stagingPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove admission permission staging file: %w", removeErr))
			return
		}
		if syncErr := syncStableCustodyPublicDirectory(parent); syncErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("flush admission permission directory: %w", syncErr))
		}
	}()
	if err := staging.Chmod(0o600); err != nil {
		return fmt.Errorf("protect admission permission staging file: %w", err)
	}
	if written, err := staging.Write(body); err != nil || written != len(body) {
		return fmt.Errorf("write admission permission staging file: %w", errors.Join(err, io.ErrShortWrite))
	}
	if err := staging.Sync(); err != nil {
		return fmt.Errorf("flush admission permission staging file: %w", err)
	}
	if err := staging.Close(); err != nil {
		return fmt.Errorf("close admission permission staging file: %w", err)
	}
	stagingOpen = false
	if err := os.Link(stagingPath, destination); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("publish admission permission: %w", err)
	}
	existing, err := readStableCustodyPrivateFile(destination)
	if err != nil || !bytes.Equal(existing, body) {
		return errors.New("admission permission destination conflicts")
	}
	return nil
}

func readStableCustodyPrivateFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != maximumAdmissionPermissionBytes {
		return nil, errors.New("admission permission is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maximumAdmissionPermissionBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(body) != maximumAdmissionPermissionBytes {
		return nil, errors.Join(readErr, closeErr, errors.New("admission permission is invalid"))
	}
	return body, nil
}
