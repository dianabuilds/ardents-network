//go:build linux

package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// All CA, leaves and keys are explicitly external fixture credentials. Product
// preparation receives only their public files and opaque private declarations.
func sourcePreparationFixture(t *testing.T, parent string, headless installedHeadless) (map[string]any, []tls.Certificate) {
	t.Helper()
	server, client, unauthorized, root, serverPin, clientPin := sourceNativeTLS(t)
	second, _, _, secondRoot, secondPin, _ := sourceNativeTLS(t)
	hexID := func(id [32]byte) string { return hex.EncodeToString(id[:]) }
	writePublic := func(name string, raw []byte) map[string]any {
		path := filepath.Join(parent, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return map[string]any{"path": path, "sha256": hexID(sha256.Sum256(raw))}
	}
	chain := func(c tls.Certificate) []byte {
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate[0]})
	}
	if headless.NetworkID == "" {
		pub := ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey)
		headless = installedHeadless{Schema: "ardents-headless-runtime-v2", NetworkStateRoot: filepath.Join(parent, "client-state"), LocalRoleStateRoot: filepath.Join(parent, "client-participation"), TimeConfidenceFile: filepath.Join(parent, "clock"), NetworkID: hexID([32]byte{8}), NetworkAuthorities: []string{hex.EncodeToString(pub)}, NetworkThreshold: 1, ClosedProfileAuthority: hex.EncodeToString(pub)}
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	p := map[string]any{"schema": "ardents-source-trust-preparation-v1", "reference_time": now, "clock_observed_at": now, "client": headless,
		"client_certificate": writePublic("client.pem", chain(client)), "client_root": writePublic("client-root.pem", root), "client_leaf_key_digest": hexID(clientPin), "client_private_key": filepath.Join(parent, "client.key"), "order_seed": hexID([32]byte{9}), "materialization_index": 0, "refresh_interval_ms": 1000}
	var members []map[string]any
	for i, item := range []struct {
		certificate tls.Certificate
		root        []byte
		pin         [32]byte
	}{{server, root, serverPin}, {second, secondRoot, secondPin}} {
		name := fmt.Sprintf("source-%d", i+1)
		members = append(members, map[string]any{"identity": hexID([32]byte{byte(71 + i)}), "family": name, "endpoint_handle": name, "address": fmt.Sprintf("127.0.0.1:%d", 19001+i), "server_name": "source-native.test", "certificate": writePublic(name+".pem", chain(item.certificate)), "root": writePublic(name+"-root.pem", item.root), "leaf_key_digest": hexID(item.pin), "private_key": filepath.Join(parent, name+".key"), "state_root": filepath.Join(parent, name+"-state"), "participation_root": filepath.Join(parent, name+"-participation")})
	}
	p["sources"] = members
	return p, []tls.Certificate{server, second, client, unauthorized}
}

func sourcePreparationJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
