//go:build !windows

package main

import (
	"os"

	"github.com/dianabuilds/ardents-network/internal/release/custody"
)

func newSecretInput() custody.SecretInput {
	return terminalSecretInput{terminal: os.Stdin, prompts: os.Stderr}
}
