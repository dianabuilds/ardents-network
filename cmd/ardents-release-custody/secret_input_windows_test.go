//go:build windows

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release/custody"
)

func TestDialogSecretInputUsesDifferentLocalPrompts(t *testing.T) {
	var messages []string
	input := dialogSecretInput{read: func(message string) ([]byte, error) {
		messages = append(messages, message)
		return []byte("release-custody-password"), nil
	}}
	if _, err := input.ReadSecret(context.Background(), custody.PromptCreate); err != nil {
		t.Fatal(err)
	}
	if _, err := input.ReadSecret(context.Background(), custody.PromptConfirm); err != nil {
		t.Fatal(err)
	}
	if _, err := input.ReadSecret(context.Background(), custody.PromptUnlock); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 || messages[0] == messages[1] || messages[1] == messages[2] || !strings.Contains(messages[2], "existing") {
		t.Fatalf("messages = %q", messages)
	}
}

func TestDialogSecretInputRejectsMissingReader(t *testing.T) {
	_, err := (dialogSecretInput{}).ReadSecret(context.Background(), custody.PromptCreate)
	if err == nil || !errors.Is(err, errDialogUnavailable) {
		t.Fatalf("ReadSecret = %v", err)
	}
}

func TestWindowsCredentialScriptUsesPasswordForm(t *testing.T) {
	script := windowsCredentialScript("Create the local Ardents release passphrase.")
	if !strings.Contains(script, "UseSystemPasswordChar=$true") || !strings.Contains(script, "$stream.Write($bytes,0,$bytes.Length)") || strings.Contains(script, "Get-Credential") {
		t.Fatalf("script does not use the local password form")
	}
}
