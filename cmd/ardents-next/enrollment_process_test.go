package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Actual compiled consumer is the exact manifest-bound program. No bundle
// artifact is launched by the verification operation itself.
func TestEnrollmentCompiledConsumerVerifiesOriginalProgramAndRefusesSubstitution(t *testing.T) {
	root := t.TempDir()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	programName, custodyName := "ardents-"+platform, "ardents-custody-"+platform
	if runtime.GOOS == "windows" {
		programName += ".exe"
		custodyName += ".exe"
	}
	programPath := filepath.Join(root, programName)
	// Share only the immutable compiled consumer; every bundle and pin is
	// independent. Rebuilding the same command here duplicates the expensive
	// full-package scan already owned by the process harness.
	program, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(programPath, program, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		programName: program, custodyName: []byte("separate custody bytes"), "1.root.json": []byte("root"),
		"timestamp.json": []byte("timestamp"), "1.snapshot.json": []byte("snapshot"), "1.targets.json": []byte("targets"),
	}
	files["RELEASE"] = []byte(strings.Join([]string{"schema=ardents-closed-alpha-enrollment-v5", "cohort=cohort", "release=release", "platform=" + platform,
		"environment=alpha", "network=network", "target_path=ardents/" + platform + "/endpoint", "artifact=" + programName, "trusted_root=1.root.json"}, "\n") + "\n")
	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		manifest.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
		if name != programName {
			if err := os.WriteFile(filepath.Join(root, name), files[name], 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	raw := []byte(manifest.String())
	if err := os.WriteFile(filepath.Join(root, "SHA256SUMS"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(raw)
	pinText := hex.EncodeToString(pin[:])
	check := func(pin string, want string, success bool) {
		t.Helper()
		command := exec.Command(programPath, "enrollment", "verify", root, pin)
		out, err := command.CombinedOutput()
		expected := "{\"outcome\":\"" + want + "\"}\n"
		if success {
			expected = fmt.Sprintf("{\"outcome\":\"verified-initial-bundle\",\"files\":%d,\"program_bytes\":%d}\n", len(files), len(program))
		}
		if (err == nil) != success || string(out) != expected {
			t.Fatalf("actual consumer: %v %q", err, out)
		}
	}
	check(pinText, "verified-initial-bundle", true)
	wrong := strings.Repeat("0", 64)
	check(wrong, "pin-mismatch", false)
	if err := os.WriteFile(filepath.Join(root, "timestamp.json"), []byte("substituted"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(pinText, "binding-mismatch", false)
}
