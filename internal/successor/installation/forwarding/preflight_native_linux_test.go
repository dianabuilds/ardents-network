//go:build network_signing_native

package forwarding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This native observation creates only its own existing destination parent.
// It neither creates an account/unit/semantic root nor starts a runtime.
func TestForwardingPreflightNativeDoesNotCreateResources(t *testing.T) {
	f := prepareInputFixture(t)
	if err := os.Mkdir(f.prefix, 0700); err != nil {
		t.Fatal("invalid environment: independent destination parent unavailable", err)
	}
	defer func() {
		if err := os.Remove(f.prefix); err != nil {
			t.Error("preflight left a destination effect", err)
		}
	}()
	input, err := ReadInputs(t.Context(), f.path)
	if err == nil {
		err = InspectInitialInputs(input)
	}
	err = errors.Join(err, input.Close())
	if err != nil {
		t.Fatal("genuine read-only initial preflight", err)
	}
	program := os.Getenv("ARDENTS_FORWARDING_NODE_PROGRAM")
	expected := os.Getenv("ARDENTS_FORWARDING_NODE_PROGRAM_SHA256")
	info, err := os.Lstat(program)
	if err != nil || !filepath.IsAbs(program) || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		t.Fatal("invalid environment: exact actual canonical Node program required", err)
	}
	raw, err := os.ReadFile(program)
	sum := sha256.Sum256(raw)
	if err != nil || len(expected) != 64 || hex.EncodeToString(sum[:]) != expected {
		t.Fatal("invalid environment: canonical Node program pin differs", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, "forwarding", "inspect-inputs", f.path)
	command.Env = append(os.Environ(), "ARDENTS_DEBUG_SOCKET="+filepath.Join(filepath.Dir(f.path), "not-a-diagnostic-socket"))
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "{\"outcome\":\"forwarding-inputs-preflight-completed\",\"installed\":false}\n" {
		t.Fatal("actual canonical read-only Forwarding preflight", err, string(output))
	}
	for _, path := range append([]string{f.request.InstallationRoot, f.request.StateRoot, f.request.ParticipationRoot, f.request.SpendRoot, f.request.HostingRoot, f.request.CredentialDirectory}, f.request.reservedSiblingPaths()...) {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("read-only preflight changed future namespace", path, err)
		}
	}
}
