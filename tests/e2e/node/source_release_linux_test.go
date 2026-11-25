//go:build linux

package node_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	releasefixture "github.com/dianabuilds/ardents-network/tests/releasefixture"
)

func TestInfrastructureCompiledRejectsOverlappingHistory(t *testing.T) {
	binary, err := os.ReadFile(buildNodeCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	dir, pin, _ := releasefixture.InfrastructureDistribution(t, t.TempDir(), binary, nil, 1, nil, "source")
	history := filepath.Join(dir, "release-history")
	cmd := exec.CommandContext(t.Context(), filepath.Join(dir, "ardents-node-linux-amd64"), "infrastructure", "authenticate-initial", dir, pin, history, "h3-test", "new-release-test", releasefixture.InfrastructureReference().Format(time.RFC3339))
	if out, err := cmd.CombinedOutput(); !commandRefused(out, err, 2, "infrastructure-paths-refused") {
		t.Fatalf("mutable history entered immutable inventory %s %v", out, err)
	}
	if _, err := os.Stat(history); !os.IsNotExist(err) {
		t.Fatal("overlapping history mutated distribution", err)
	}
}

func TestInfrastructureCompiledRejectsRetiredSourceOperations(t *testing.T) {
	program := buildNodeCommand(t)
	for _, operation := range []string{"preflight-initial", "preflight-candidate", "install-initial-stopped"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "absent")
			cmd := exec.CommandContext(t.Context(), program, "infrastructure", operation, path)
			out, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 || len(out) != 0 {
				t.Fatalf("retired operation returned %q, %v", out, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("retired operation changed input path", err)
			}
		})
	}
}

// Independent oracle: literal ordered schemas, independently spelled names and
// ordinary 3-of-5 signatures, without product preparation/descriptor codecs.
func TestInfrastructureCompiledCommandInitialCandidateAndForeignArtifact(t *testing.T) {
	command := buildNodeCommand(t)
	binary, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	dir, pin, files := releasefixture.InfrastructureDistribution(t, t.TempDir(), binary, nil, 1, nil, "source")
	program := filepath.Join(dir, "ardents-node-linux-amd64")
	history := filepath.Join(t.TempDir(), "history")
	run := func(executable string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(t.Context(), executable, append([]string{"infrastructure"}, args...)...)
		return cmd.CombinedOutput()
	}
	wantReceipt := func(programOutcome string) []byte {
		return fmt.Appendf(nil, "{\"outcome\":\"authenticated-source-artifact\",\"program_outcome\":%q,\"generation_outcome\":\"no-update\",\"program_sha256\":\"%x\",\"descriptor_sha256\":\"%x\",\"template_sha256\":\"%x\"}\n",
			programOutcome, sha256.Sum256(binary), sha256.Sum256(files["infrastructure-source.json"]), sha256.Sum256(files["ardents-source.service"]))
	}

	args := []string{"authenticate-initial", dir, pin, history, "h3-test", "new-release-test", releasefixture.InfrastructureReference().Format(time.RFC3339)}
	if out, err := run(command, args...); !commandRefused(out, err, 1, "infrastructure-inventory-refused") {
		t.Fatalf("foreign actual executable accepted %s %v", out, err)
	}
	if _, err := os.Stat(history); !os.IsNotExist(err) {
		t.Fatal("foreign artifact mutated history")
	}
	if out, err := run(program, args...); err != nil || !bytes.Equal(out, wantReceipt("release-accepted")) {
		t.Fatalf("actual self initial %s %v", out, err)
	}
	if out, err := run(program, args...); err == nil {
		t.Fatalf("initial pin reused for retained history %s", out)
	}
	candidate := []string{"authenticate-candidate", dir, history, "h3-test", "new-release-test", releasefixture.InfrastructureReference().Format(time.RFC3339)}
	if out, err := run(command, candidate...); err != nil || !bytes.Equal(out, wantReceipt("no-update")) {
		t.Fatalf("retained candidate %s %v", out, err)
	}
	cold := append([]string(nil), candidate...)
	cold[2] = filepath.Join(t.TempDir(), "missing-history")
	if out, err := run(program, cold...); err == nil {
		t.Fatalf("candidate cold trust %s", out)
	}
	wrong := append([]string(nil), args...)
	wrong[2] = strings.Repeat("0", 64)
	wrong[3] = filepath.Join(t.TempDir(), "wrong-pin-history")
	if out, err := run(program, wrong...); err == nil {
		t.Fatalf("wrong pin %s", out)
	}
	if _, err := os.Stat(wrong[3]); !os.IsNotExist(err) {
		t.Fatal("wrong pin mutated history")
	}
	wrong = append([]string(nil), args...)
	wrong[4] = "foreign-environment"
	if out, err := run(program, wrong...); err == nil {
		t.Fatalf("wrong local %s", out)
	}
	wrong = append([]string(nil), args...)
	wrong[6] = releasefixture.InfrastructureReference().Add(500 * 24 * time.Hour).Format(time.RFC3339)
	if out, err := run(program, wrong...); err == nil {
		t.Fatalf("expiry %s", out)
	}
}

func commandRefused(out []byte, err error, code int, outcome string) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == code &&
		bytes.Equal(out, fmt.Appendf(nil, "{\"outcome\":%q}\n", outcome))
}
