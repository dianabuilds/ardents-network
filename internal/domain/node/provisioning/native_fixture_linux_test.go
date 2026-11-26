//go:build linux && network_signing_native

package provisioning

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func sourceNativePinnedProgram(t *testing.T, variable string) (string, []byte) {
	t.Helper()
	path, pin := os.Getenv(variable), os.Getenv(variable+"_SHA256")
	raw, err := os.ReadFile(path)
	digest := sha256.Sum256(raw)
	if err != nil || !filepath.IsAbs(path) || len(pin) != 64 || hex.EncodeToString(digest[:]) != pin {
		t.Fatal("invalid environment: original independently pinned program required", variable, err)
	}
	return path, raw
}

func provisionCommand(t *testing.T, program string, args ...string) *exec.Cmd {
	return exec.CommandContext(t.Context(), program, append([]string{"infrastructure", "provision"}, args...)...)
}

func sourceArtifactNativeParent(t *testing.T) string {
	t.Helper()
	return networkfixture.ProtectedExt4Parent(t)
}
