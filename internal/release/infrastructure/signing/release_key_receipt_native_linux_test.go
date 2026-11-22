//go:build linux && network_signing_native

package signing

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestControlReleaseKeyReceiptsNative(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	program, _ := sourceNativePinnedProgram(t, "ARDENTS_INFRASTRUCTURE_RELEASE_PROGRAM")
	seen := make(map[[ed25519.PublicKeySize]byte]bool)
	for _, role := range []string{"source", "forwarding", "join", "descriptor"} {
		t.Run(role, func(t *testing.T) {
			output := filepath.Join(parent, role)
			call := func() ([]byte, error) {
				return exec.CommandContext(t.Context(), program, role, "prepare-release-keys", output).Output()
			}
			raw, err := call()
			if err != nil {
				t.Fatal("role failed to create its Release keys", err)
			}
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 5 {
				t.Fatal("unexpected inventory of Release keys", err)
			}
			var want bytes.Buffer
			fmt.Fprintf(&want, `{"operation":%q,"ordinary_threshold":3,"independent_custody":false,"keys":[`, role+".prepare-release-keys")
			for i := 1; i <= 5; i++ {
				name := fmt.Sprintf("release-key-%d.pem", i)
				public := releaseReceiptFixturePublicKey(t, filepath.Join(output, name))
				if seen[public] {
					t.Fatal("roles or slots share a Release key")
				}
				seen[public] = true
				if i > 1 {
					want.WriteByte(',')
				}
				fmt.Fprintf(&want, `{"file":%q,"ed25519_public_hex":"%x","public_sha256":"%x"}`, name, public, sha256.Sum256(public[:]))
			}
			want.WriteString("]}\n")
			if !bytes.Equal(raw, want.Bytes()) {
				t.Fatal("public Release key receipt differs from the generated files")
			}
			if retry, err := call(); err == nil || len(retry) != 0 {
				t.Fatal("occupied output supplied another Release key receipt", err)
			}
		})
	}
}

// The oracle derives the public key from each newly generated private test file.
func releaseReceiptFixturePublicKey(t *testing.T, path string) [ed25519.PublicKeySize]byte {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || info.Mode() != 0600 {
		t.Fatal("Release fixture key is missing or has unexpected access", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
		t.Fatal("Release fixture key format differs")
	}
	defer clear(block.Bytes)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok || len(key) != ed25519.PrivateKeySize {
		t.Fatal("Release fixture key is not an Ed25519 PKCS#8 key", err)
	}
	defer clear(key)
	return [ed25519.PublicKeySize]byte(key.Public().(ed25519.PublicKey))
}
