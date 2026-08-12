package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

type rawConfig struct {
	root, network, authorities, at, epoch, inputs, material string
	threshold                                               int
}

func (raw rawConfig) networkStateConfig() (state.Config, error) {
	var networkID [32]byte
	if err := planfile.FixedHex(raw.network, networkID[:]); err != nil {
		return state.Config{}, fmt.Errorf("network-id: %w", err)
	}
	authorities := make(map[[32]byte]ed25519.PublicKey)
	for _, encoded := range strings.Split(raw.authorities, ",") {
		if encoded == "" {
			return state.Config{}, errors.New("authorities are required")
		}
		public := make([]byte, ed25519.PublicKeySize)
		if err := planfile.FixedHex(encoded, public); err != nil {
			return state.Config{}, fmt.Errorf("authority: %w", err)
		}
		authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	at, err := time.Parse(time.RFC3339, raw.at)
	if err != nil {
		return state.Config{}, fmt.Errorf("at: %w", err)
	}
	return state.Config{Root: raw.root, NetworkID: networkID, Authorities: authorities, Threshold: raw.threshold, Now: at}, nil
}
