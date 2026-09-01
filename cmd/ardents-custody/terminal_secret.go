package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/custody"
	"golang.org/x/term"
)

// terminalSecretInput confines interactive secret entry to a real terminal.
// It deliberately does not fall back to an argument, environment variable, or
// a stream shared with application data.
type terminalSecretInput struct {
	terminal *os.File
	prompts  io.Writer
}

func (input terminalSecretInput) ReadSecret(ctx context.Context, prompt custody.SecretPrompt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.terminal == nil || !term.IsTerminal(int(input.terminal.Fd())) {
		return nil, errors.New("custody secret input requires an interactive terminal")
	}
	fmt.Fprintf(input.prompts, "%s password: ", prompt)
	value, err := term.ReadPassword(int(input.terminal.Fd()))
	fmt.Fprintln(input.prompts)
	if err != nil {
		return nil, fmt.Errorf("read terminal password: %w", err)
	}
	return value, nil
}

func (input terminalSecretInput) Confirm(ctx context.Context, prompt custody.ConfirmationPrompt) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if input.terminal == nil || !term.IsTerminal(int(input.terminal.Fd())) {
		return false, errors.New("custody confirmation requires an interactive terminal")
	}
	confirmation := "replace"
	if prompt == custody.ConfirmationPromptVaultPurge {
		confirmation = "purge"
	}
	fmt.Fprintf(input.prompts, "confirm %s by typing %s: ", prompt, confirmation)
	line, err := bufio.NewReader(input.terminal).ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read terminal confirmation: %w", err)
	}
	return strings.TrimSpace(line) == confirmation, nil
}

func (input terminalSecretInput) ReadServiceRequestCommitment(ctx context.Context) ([32]byte, error) {
	if err := ctx.Err(); err != nil {
		return [32]byte{}, err
	}
	if input.terminal == nil || !term.IsTerminal(int(input.terminal.Fd())) {
		return [32]byte{}, errors.New("service request commitment requires an interactive terminal")
	}
	fmt.Fprint(input.prompts, "service-request SHA-256 from the requesting host: ")
	line, err := bufio.NewReader(input.terminal).ReadString('\n')
	if err != nil {
		return [32]byte{}, fmt.Errorf("read service request commitment: %w", err)
	}
	text := strings.TrimSpace(line)
	decoded, err := hex.DecodeString(text)
	if err != nil || len(text) != 64 || len(decoded) != 32 || hex.EncodeToString(decoded) != text {
		return [32]byte{}, errors.New("service request commitment must be lowercase SHA-256")
	}
	var commitment [32]byte
	copy(commitment[:], decoded)
	return commitment, nil
}
