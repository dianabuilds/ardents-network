package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

type rawConfig struct {
	root        string
	network     string
	authorities string
	threshold   int
	at          string
	epoch       string
	inputs      string
	material    string
}

func (raw rawConfig) networkStateConfig() (networkstate.Config, error) {
	var networkID [32]byte
	if err := decodeFixedHex(raw.network, networkID[:]); err != nil {
		return networkstate.Config{}, fmt.Errorf("network-id: %w", err)
	}
	authorities := make(map[[32]byte]ed25519.PublicKey)
	for _, encoded := range strings.Split(raw.authorities, ",") {
		if encoded == "" {
			return networkstate.Config{}, errors.New("authorities are required")
		}
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeFixedHex(encoded, public); err != nil {
			return networkstate.Config{}, fmt.Errorf("authority: %w", err)
		}
		id := sha256.Sum256(public)
		authorities[id] = ed25519.PublicKey(public)
	}
	at, err := time.Parse(time.RFC3339, raw.at)
	if err != nil {
		return networkstate.Config{}, fmt.Errorf("at: %w", err)
	}
	return networkstate.Config{
		Root: raw.root, NetworkID: networkID, Authorities: authorities,
		Threshold: raw.threshold, Now: at,
	}, nil
}

func decodeFixedHex(encoded string, destination []byte) error {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return err
	}
	if len(decoded) != len(destination) {
		return fmt.Errorf("decoded length is %d, want %d", len(decoded), len(destination))
	}
	copy(destination, decoded)
	return nil
}
