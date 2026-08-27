//go:build !windows

package main

import (
	"os"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func newSecretInput() state.AlphaGenesisSecretInput {
	return terminalSecretInput{terminal: os.Stdin, prompts: os.Stderr}
}
