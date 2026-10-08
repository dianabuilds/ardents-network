//go:build linux

package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Actual program/worker/unit bytes are necessary for installed startup. This
// checks their genuine signed byte-authentication consumer only; no manager,
// root completion peer or accepting worker is substituted by this fixture.
func TestExecutionSignedInstalledResources(t *testing.T) {
	worker := filepath.Join(t.TempDir(), "ardents-text")
	build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-o", worker, "github.com/dianabuilds/ardents-network/cmd/ardents-text")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build genuine worker: %v: %s", err, out)
	}
	resources := make(map[string][]byte)
	var err error
	resources["ardents-text-linux-amd64"], err = os.ReadFile(worker)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"} {
		resources[name], err = os.ReadFile(filepath.Join("..", "..", "packaging", "text-worker", name))
		if err != nil {
			t.Fatal(err)
		}
	}
	authority := newConsumerReleaseAuthority(t, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC).Add(365*24*time.Hour))
	bundle, pin, program := signedConsumerRelease(t, "linux-amd64", true, true, 1, resources, authority, 1, 1)
	request := filepath.Join(t.TempDir(), "request.json")
	history := filepath.Join(t.TempDir(), "floors")
	if err := os.WriteFile(request, installationRequestBytes(t, bundle, pin, history), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), program, "installation", "authenticate-initial", "--request", request)
	out, err := command.CombinedOutput()
	var result struct {
		Outcome string `json:"outcome"`
	}
	if err != nil || json.Unmarshal(out, &result) != nil || result.Outcome != "authenticated-generation" {
		t.Fatalf("genuine installed byte pair refused: %v: %s", err, out)
	}
	candidateResources := make(map[string][]byte, len(resources))
	for name, body := range resources {
		candidateResources[name] = append([]byte(nil), body...)
	}
	candidateResources["ardents-endpoint.service"] = append([]byte("# signed successor fixture\n"), candidateResources["ardents-endpoint.service"]...)
	candidate, _, _ := signedConsumerRelease(t, "linux-amd64", true, true, 2, candidateResources, authority, 2, 2)
	command = exec.CommandContext(t.Context(), program, "installation", "authenticate-candidate", candidate, history, "2030-01-02T03:04:05Z")
	out, err = command.CombinedOutput()
	if err != nil || json.Unmarshal(out, &result) != nil || result.Outcome != "authenticated-generation" {
		t.Fatalf("genuine retained-Root candidate refused: %v: %s", err, out)
	}
	command = exec.CommandContext(t.Context(), program, "installation", "authenticate-candidate", bundle, history, "2030-01-02T03:04:05Z")
	if out, err := command.CombinedOutput(); err == nil {
		t.Fatalf("accepted old signed snapshot after candidate floors: %s", out)
	}
	// Change actual worker bytes while retaining the independently delivered pin
	// and signed inventory. Refusal must precede a fresh Release history effect.
	changed := append([]byte(nil), resources["ardents-text-linux-amd64"]...)
	changed[0] ^= 1
	if err := os.WriteFile(filepath.Join(bundle, "ardents-text-linux-amd64"), changed, 0600); err != nil {
		t.Fatal(err)
	}
	absentHistory := filepath.Join(t.TempDir(), "absent-floors")
	if err := os.WriteFile(request, installationRequestBytes(t, bundle, pin, absentHistory), 0600); err != nil {
		t.Fatal(err)
	}
	command = exec.CommandContext(t.Context(), program, "installation", "authenticate-initial", "--request", request)
	if out, err := command.CombinedOutput(); err == nil {
		t.Fatalf("accepted substituted actual worker: %s", out)
	}
	if _, err := os.Lstat(absentHistory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("worker substitution reached fresh Release history", err)
	}
}
