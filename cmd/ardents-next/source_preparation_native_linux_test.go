//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
	sourcecommand "github.com/dianabuilds/ardents-network/internal/network/source/command"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

func writeSourcePreparationPlan(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func sourcePreparationArguments(t *testing.T) (map[string]any, string, string) {
	t.Helper()
	parent := signingNativeParent(t)
	pub := ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey)
	client := endpoint.HeadlessPlan{
		Schema: "ardents-headless-runtime-v2", NetworkStateRoot: filepath.Join(parent, "client-state"),
		LocalRoleStateRoot: filepath.Join(parent, "client-participation"), TimeConfidenceFile: filepath.Join(parent, "clock"),
		NetworkID: hex.EncodeToString(append([]byte{8}, make([]byte, 31)...)), NetworkThreshold: 1,
		NetworkAuthorities: []string{hex.EncodeToString(pub)}, ClosedProfileAuthority: hex.EncodeToString(pub),
	}
	p, _ := sourcefixture.TrustPreparationFixture(t, parent, client)
	plan, output := filepath.Join(parent, "plan.json"), filepath.Join(parent, "output")
	writeSourcePreparationPlan(t, plan, p)
	return p, plan, output
}

func TestSourcePreparationNativePublicOnlyCommand(t *testing.T) {
	p, plan, output := sourcePreparationArguments(t)
	// None of the three private declarations exists. Successful preparation
	// cannot rely on even opening them, let alone generating or copying keys.
	cmd := exec.CommandContext(t.Context(), compiledCommand(t), "source", "prepare-trust", plan, output)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal("genuine preparation command", err)
	}
	var receipt struct {
		Schema, Validation string
		Files              map[string]string
	}
	if json.Unmarshal(raw, &receipt) != nil || receipt.Schema != "ardents-source-trust-prepared-v1" || len(receipt.Files) != 10 {
		t.Fatal("public receipt inventory", string(raw))
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 10 {
		t.Fatal("finite inventory", err)
	}
	for _, path := range []string{p["client_private_key"].(string), p["sources"].([]map[string]any)[0]["private_key"].(string), p["sources"].([]map[string]any)[1]["private_key"].(string)} {
		if bytes.Contains(raw, []byte(path)) {
			t.Fatal("private path in receipt")
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("private path effect", err)
		}
	}
	client := p["client"].(endpoint.HeadlessPlan)
	roots := []string{client.NetworkStateRoot, client.LocalRoleStateRoot}
	for _, member := range p["sources"].([]map[string]any) {
		roots = append(roots, member["state_root"].(string), member["participation_root"].(string))
	}
	for _, root := range roots {
		if _, e := os.Lstat(root); !errors.Is(e, os.ErrNotExist) {
			t.Fatal("offline State/participation root effect", e)
		}
	}
	if sourcecommand.Run(t.Context(), []string{"prepare-trust", plan, output}, io.Discard, io.Discard) == 0 {
		t.Fatal("adopted output")
	}
}

func TestSourcePreparationNativeReceiptFailure(t *testing.T) {
	_, plan, output := sourcePreparationArguments(t)
	args := []string{"prepare-trust", plan, output}
	if sourcecommand.Run(t.Context(), args, sourcePreparationFailWriter{}, io.Discard) == 0 {
		t.Fatal("receipt failure accepted")
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal("receipt did not follow physical publication", err)
	}
	if sourcecommand.Run(t.Context(), args, io.Discard, io.Discard) == 0 {
		t.Fatal("failed receipt output adopted")
	}
}

type sourcePreparationFailWriter struct{}

func (sourcePreparationFailWriter) Write([]byte) (int, error) { return 0, syscall.EPIPE }
