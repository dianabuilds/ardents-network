//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/release/custody"
)

type dialogSecretInput struct {
	read func(string) ([]byte, error)
}

var errDialogUnavailable = errors.New("release custody graphical secret input is unavailable")

func newSecretInput() custody.SecretInput {
	return dialogSecretInput{read: readWindowsPassphrase}
}

func (input dialogSecretInput) ReadSecret(ctx context.Context, prompt custody.Prompt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.read == nil {
		return nil, errDialogUnavailable
	}
	message := "Create the local Ardents release passphrase."
	if prompt == custody.PromptConfirm {
		message = "Confirm the local Ardents release passphrase."
	}
	return input.read(message)
}

func readWindowsPassphrase(message string) ([]byte, error) {
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-STA", "-Command", windowsCredentialScript(message))
	value, err := command.CombinedOutput()
	if err != nil {
		diagnostic := strings.TrimSpace(string(value))
		if diagnostic == "" {
			return nil, fmt.Errorf("read release custody passphrase through local dialog: %w", err)
		}
		return nil, fmt.Errorf("read release custody passphrase through local dialog: %s", diagnostic)
	}
	return value, nil
}

func windowsCredentialScript(message string) string {
	if message == "Confirm the local Ardents release passphrase." {
		return "$ErrorActionPreference='Stop';$credential=Get-Credential -UserName 'release-custody' -Message 'Confirm the local Ardents release passphrase.';if($null -eq $credential){exit 3};$pointer=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($credential.Password);try{$plain=[Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer);[Console]::OpenStandardOutput().Write([Text.Encoding]::UTF8.GetBytes($plain))}finally{[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer)}"
	}
	return "$ErrorActionPreference='Stop';$credential=Get-Credential -UserName 'release-custody' -Message 'Create the local Ardents release passphrase.';if($null -eq $credential){exit 3};$pointer=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($credential.Password);try{$plain=[Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer);[Console]::OpenStandardOutput().Write([Text.Encoding]::UTF8.GetBytes($plain))}finally{[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer)}"
}
