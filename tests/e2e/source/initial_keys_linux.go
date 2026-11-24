//go:build linux && network_signing_native

package source

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func InitialKeyArguments(t *testing.T, parent string, count int) (string, string) {
	t.Helper()
	plan, root := filepath.Join(parent, "plan.json"), filepath.Join(parent, "keys")
	raw := fmt.Sprintf(`{"schema":"ardents-initial-keys-plan-v1","operation":"prepare-initial-keys","node_count":%d}`, count)
	if e := os.WriteFile(plan, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	return plan, root
}

// Independent stdlib reader verifies every exact receipt entry, full canonical
// PKCS8 encoding, derived key, native mode and closed inventory.
func VerifyInitialKeys(t *testing.T, root string, count int, stdout []byte) []ed25519.PrivateKey {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join(root, "public-receipt.json"))
	if e != nil {
		t.Fatal(e)
	}
	if stdout != nil && !bytes.Equal(raw, stdout) {
		t.Fatal("receipt mismatch")
	}
	var receipt struct {
		Schema      string `json:"schema"`
		Operation   string `json:"operation"`
		Signed      bool   `json:"signed"`
		State       bool   `json:"state_accepted"`
		Installed   bool   `json:"installed_qualification"`
		Independent bool   `json:"independent_custody"`
		Keys        []struct {
			Purpose string `json:"purpose"`
			Index   *int   `json:"index"`
			File    string `json:"file"`
			Public  string `json:"ed25519_public_hex"`
			Digest  string `json:"public_sha256"`
		} `json:"keys"`
	}
	if json.Unmarshal(raw, &receipt) != nil || len(receipt.Keys) != count+1 || receipt.Schema != "ardents-initial-keys-receipt-v1" || receipt.Operation != "prepare-initial-keys" || receipt.State || receipt.Signed || receipt.Installed || receipt.Independent {
		t.Fatal("receipt claims")
	}
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) != count+2 {
		t.Fatal("inventory", e)
	}
	seen := map[string]bool{}
	var keys []ed25519.PrivateKey
	for i, k := range receipt.Keys {
		want := "state-key.pem"
		purpose := "state"
		if i > 0 {
			want = fmt.Sprintf("node-%02d-key.pem", i-1)
			purpose = "node"
		}
		if k.File != want || k.Purpose != purpose || (i == 0 && k.Index != nil) || (i > 0 && (k.Index == nil || *k.Index != i-1)) {
			t.Fatal("role/index inventory")
		}
		private, e := os.ReadFile(filepath.Join(root, k.File))
		if e != nil {
			t.Fatal(e)
		}
		block, rest := pem.Decode(private)
		if block == nil || len(rest) != 0 || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 {
			t.Fatal("PEM")
		}
		parsed, e := x509.ParsePKCS8PrivateKey(block.Bytes)
		if e != nil {
			t.Fatal("PKCS8")
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok {
			t.Fatal("algorithm")
		}
		der, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil || !bytes.Equal(der, block.Bytes) || !bytes.Equal(private, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})) {
			t.Fatal("canonical bytes")
		}
		derived := ed25519.NewKeyFromSeed(key[:32])
		if !bytes.Equal(derived, key) {
			t.Fatal("seed consistency")
		}
		clear(derived)
		pub := key.Public().(ed25519.PublicKey)
		digest := sha256.Sum256(pub)
		if k.Public != hex.EncodeToString(pub) || k.Digest != hex.EncodeToString(digest[:]) || seen[k.Public] {
			t.Fatal("public facts/distinctness")
		}
		seen[k.Public] = true
		info, e := os.Lstat(filepath.Join(root, k.File))
		if e != nil || !artifactio.ProtectedFileMode(info, true) {
			t.Fatal("native private file", e)
		}
		clear(private)
		clear(block.Bytes)
		clear(der)
		keys = append(keys, key)
	}
	for _, key := range keys {
		t.Cleanup(func() { clear(key) })
	}
	return keys
}
