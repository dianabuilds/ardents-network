//go:build live

package network_test

import (
	"encoding/hex"
	"path/filepath"
	"testing"
)

func writeBlockedProbeInputs(t *testing.T, root string, certificate, envelope []byte, identity [32]byte) {
	t.Helper()
	directory := filepath.Join(root, "input", "probe")
	writeLiveFile(t, filepath.Join(directory, "front-cert.pem"), certificate)
	writeLivePlan(t, directory, "probe", map[string]any{
		"address":     "203.0.113.8:8480",
		"server_name": "front.example",
		"path":        "/entry",
		"envelope":    hex.EncodeToString(envelope),
		"identity":    hex.EncodeToString(identity[:]),
	})
}
