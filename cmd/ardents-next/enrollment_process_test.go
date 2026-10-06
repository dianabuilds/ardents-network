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
	programName, controlName := "ardents-"+platform, "ardents-control-"+platform
	if runtime.GOOS == "windows" {
		programName += ".exe"
		controlName += ".exe"
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
	files := map[string][]byte{programName: program, controlName: []byte("separate control bytes"), "1.root.json": []byte("root"),
		"catalog.ac1": []byte("catalog"), "catalog.pub": []byte("disclosure"), "release.ac1": []byte("release"),
		"network.ac1": []byte("network"), "compatibility.ac1": []byte("compatibility"), "release.pub": []byte("release root"),
		"network.pub": []byte("network root"), "compatibility.pub": []byte("compatibility root"), "corpus.pub": []byte("corpus"), "timestamp.json": []byte("timestamp")}
	files["RELEASE"] = []byte(strings.Join([]string{"schema=ardents-closed-alpha-enrollment-v3", "cohort=cohort", "release=release", "platform=" + platform,
		"environment=alpha", "network=network", "target_path=ardents/" + platform + "/endpoint", "artifact=" + programName,
		"trusted_root=1.root.json", "control_catalog=catalog.ac1", "disclosure_root=catalog.pub", "control_release=release.ac1", "control_network=network.ac1",
		"control_compatibility=compatibility.ac1", "control_release_root=release.pub", "control_network_root=network.pub", "control_compatibility_root=compatibility.pub",
		"corpus_authority=corpus.pub", "control_artifact=" + controlName}, "\n") + "\n")
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
