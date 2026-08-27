package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/release/custody"
	"golang.org/x/term"
)

type terminalSecretInput struct {
	terminal *os.File
	prompts  io.Writer
}

func (input terminalSecretInput) ReadSecret(ctx context.Context, prompt custody.Prompt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.terminal == nil || !term.IsTerminal(int(input.terminal.Fd())) {
		return nil, errors.New("release custody secret input requires an interactive terminal")
	}
	fmt.Fprintf(input.prompts, "%s passphrase: ", prompt)
	value, err := term.ReadPassword(int(input.terminal.Fd()))
	fmt.Fprintln(input.prompts)
	if err != nil {
		return nil, fmt.Errorf("read release custody passphrase: %w", err)
	}
	return value, nil
}
