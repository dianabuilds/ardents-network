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
	return fmt.Sprintf("Add-Type -AssemblyName System.Windows.Forms;$form=New-Object System.Windows.Forms.Form;$form.Text='Ardents release custody';$form.StartPosition='CenterScreen';$form.Width=500;$form.Height=190;$label=New-Object System.Windows.Forms.Label;$label.Left=16;$label.Top=18;$label.Width=450;$label.Text='%s';$box=New-Object System.Windows.Forms.TextBox;$box.Left=16;$box.Top=56;$box.Width=450;$box.UseSystemPasswordChar=$true;$ok=New-Object System.Windows.Forms.Button;$ok.Text='Continue';$ok.Left=300;$ok.Top=100;$ok.Width=80;$cancel=New-Object System.Windows.Forms.Button;$cancel.Text='Cancel';$cancel.Left=386;$cancel.Top=100;$cancel.Width=80;$form.Controls.AddRange(@($label,$box,$ok,$cancel));$form.AcceptButton=$ok;$form.CancelButton=$cancel;$ok.Add_Click({$form.Tag=$box.Text;$form.DialogResult=[System.Windows.Forms.DialogResult]::OK;$form.Close()});$cancel.Add_Click({$form.DialogResult=[System.Windows.Forms.DialogResult]::Cancel;$form.Close()});$form.Add_Shown({$box.Select()});if($form.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK){exit 3};$bytes=[Text.Encoding]::UTF8.GetBytes([string]$form.Tag);$stream=[Console]::OpenStandardOutput();$stream.Write($bytes,0,$bytes.Length)", message)
}
