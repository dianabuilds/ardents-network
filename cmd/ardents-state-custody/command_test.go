package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type commandSecretInput struct{}

func (commandSecretInput) ReadSecret(context.Context, state.AlphaGenesisPrompt) ([]byte, error) {
	return nil, errors.New("command test secret input must stay behind the Module")
}

func TestRunInitializesOnlyFixedAlphaGenesisAndRendersPublicReceipt(t *testing.T) {
	root := filepath.Join(t.TempDir(), "custody")
	var output bytes.Buffer
	networkID := sha256.Sum256([]byte("network"))
	authority := sha256.Sum256([]byte("authority"))
	epoch := sha256.Sum256([]byte("epoch"))
	envelope := sha256.Sum256([]byte("envelope"))
	now := time.Unix(1_800_100_000, 0).UTC()
	called := false
	err := runWithInitializer(context.Background(), []string{"initialize-alpha-genesis", "--root", root}, &output,
		commandSecretInput{}, func(_ context.Context, config state.AlphaGenesisConfig, _ state.AlphaGenesisSecretInput) (state.AlphaGenesisReceipt, error) {
			called = true
			if config.Root != root {
				t.Fatalf("unexpected root %q", config.Root)
			}
			return state.AlphaGenesisReceipt{EnvelopeDigest: envelope, NetworkID: networkID, AuthorityPublic: authority,
				EpochDigest: epoch, Profile: "ardents-interactive-route-v1", Threshold: 1,
				NotBefore: now, NotAfter: now.Add(30 * 24 * time.Hour)}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("State Module was not called")
	}
	var receipt struct {
		Schema    string   `json:"schema"`
		Threshold uint8    `json:"threshold"`
		Epoch     []byte   `json:"epoch"`
		Inputs    [][]byte `json:"inputs"`
		Materials [][]byte `json:"materials"`
	}
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Schema != "ardents-functional-alpha-state-receipt-v1" || receipt.Threshold != 1 ||
		len(receipt.Inputs) != 0 || len(receipt.Materials) != 0 {
		t.Fatalf("unexpected command receipt: %+v", receipt)
	}
}

func TestRunRejectsUnknownOperationAndRelativeRoot(t *testing.T) {
	for _, arguments := range [][]string{{}, {"initialize"}, {"initialize-alpha-genesis", "--root", "relative"},
		{"initialize-alpha-genesis", "--root", filepath.Join(t.TempDir(), "root"), "extra"}} {
		called := false
		err := runWithInitializer(context.Background(), arguments, &bytes.Buffer{}, commandSecretInput{},
			func(context.Context, state.AlphaGenesisConfig, state.AlphaGenesisSecretInput) (state.AlphaGenesisReceipt, error) {
				called = true
				return state.AlphaGenesisReceipt{}, nil
			})
		if err == nil || called {
			t.Fatalf("arguments %v returned %v with called=%v", arguments, err, called)
		}
	}
}
