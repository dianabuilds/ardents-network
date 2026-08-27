package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release/custody"
)

func TestRunRejectsInvalidArgumentsBeforeSecretInput(t *testing.T) {
	err := run(context.Background(), []string{"initialize"}, &bytes.Buffer{}, unreadInput{})
	if err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Fatalf("run = %v", err)
	}
}

func TestRunRejectsInvalidInspectArgumentsBeforeSecretInput(t *testing.T) {
	err := run(context.Background(), []string{"inspect"}, &bytes.Buffer{}, unreadInput{})
	if err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Fatalf("run = %v", err)
	}
}

func TestRunRejectsInvalidAssembleArgumentsBeforeSecretInput(t *testing.T) {
	err := run(context.Background(), []string{"assemble", "--root", "relative"}, &bytes.Buffer{}, unreadInput{})
	if err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Fatalf("run = %v", err)
	}
}

func TestReadBoundedCommandFileRejectsNonFileAndOversizedInput(t *testing.T) {
	if _, err := readBoundedCommandFile(t.TempDir(), 8); err == nil {
		t.Fatal("directory input was accepted")
	}
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBoundedCommandFile(path, 8); err == nil {
		t.Fatal("oversized input was accepted")
	}
}

func TestRenderAlphaInputsReceiptContainsOnlyPublicEvidence(t *testing.T) {
	var output bytes.Buffer
	receipt := custody.AlphaInputsReceipt{Cohort: "h4-alpha-1", Release: "h4-alpha-1-rc-1",
		SourceRevision: strings.Repeat("a", 40), Preflight: "accepted", TUFVersion: 1, CatalogVersion: 1,
		Files: []custody.AlphaInputFile{{Name: "1.root.json", Size: 10, Digest: [32]byte{1}}}}
	if err := renderAlphaInputsReceipt(&output, receipt); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "ardents-alpha-inputs-receipt-v1") || !strings.Contains(output.String(), `"preflight":"accepted"`) ||
		strings.Contains(output.String(), "private") || strings.Contains(output.String(), "passphrase") {
		t.Fatalf("receipt = %q", output.String())
	}
}

func TestRunInitializesRecordAndRendersOnlyReceipt(t *testing.T) {
	root := t.TempDir()
	var output bytes.Buffer
	password := []byte("release-custody-password")
	err := run(context.Background(), []string{"initialize", "--root", root}, &output, &fixedInput{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), string(password)) || !strings.Contains(output.String(), "ardents-release-custody-receipt-v1") {
		t.Fatalf("receipt = %q", output.String())
	}
}

func TestRunInspectsExistingRecordWithOneSecret(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	if err := run(context.Background(), []string{"initialize", "--root", root}, &bytes.Buffer{}, &fixedInput{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"inspect", "--root", root}, &output, &fixedInput{values: [][]byte{password}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), string(password)) || !strings.Contains(output.String(), "ardents-release-custody-receipt-v1") {
		t.Fatalf("receipt = %q", output.String())
	}
}

type unreadInput struct{}

func (unreadInput) ReadSecret(context.Context, custody.Prompt) ([]byte, error) {
	return nil, errors.New("secret read should not occur")
}

type fixedInput struct {
	values [][]byte
	next   int
}

func (input *fixedInput) ReadSecret(_ context.Context, _ custody.Prompt) ([]byte, error) {
	if input.next >= len(input.values) {
		return nil, errors.New("unexpected secret request")
	}
	value := append([]byte(nil), input.values[input.next]...)
	input.next++
	return value, nil
}
