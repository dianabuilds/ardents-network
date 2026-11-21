package enrollment

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Exercise the actual repository packaging entry points. These fixtures contain
// inert program/resource bytes and prove static inventory behavior, not initial
// pin provenance, signed Release acceptance or installed platform support.
func TestPackagingConsumersPreserveArtifactAndProtectedInventory(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	build := func(name string) string {
		t.Helper()
		binary := filepath.Join(t.TempDir(), name)
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		cmd := exec.CommandContext(t.Context(), "go", "build", "-buildvcs=false", "-o", binary, "./scripts/"+name+".go")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build packaging consumer: %v\n%s", err, output)
		}
		return binary
	}
	identity := build("enrollment-artifact-name")
	for _, test := range []struct{ command, platform, want string }{
		{"ardents", "linux-amd64", "ardents-linux-amd64\n"},
		{"ardents-control", "windows-amd64", "ardents-control-windows-amd64.exe\n"},
		{"ardents-node", "windows-arm64", "ardents-node-windows-arm64.exe\n"},
		{"ardents-custody", "darwin-arm64", "ardents-custody-darwin-arm64\n"},
	} {
		output, err := exec.CommandContext(t.Context(), identity, test.command, test.platform).Output()
		if err != nil || string(output) != test.want {
			t.Fatalf("artifact identity changed: %q / %v", output, err)
		}
	}
	checker := build("protected-generation-check")
	run := func(ctx context.Context, root, release string) error {
		return exec.CommandContext(ctx, checker, root, filepath.Join(root, "ardents-linux-amd64"), filepath.Join(root, "ardents-text-linux-amd64"), release).Run()
	}
	for _, failure := range []string{"none", "foreign-release", "changed-resource", "partial"} {
		t.Run(failure, func(t *testing.T) {
			r, files := protectedFixture(t)
			writeFixture(t, &r, files)
			release := "release"
			switch failure {
			case "foreign-release":
				release = "another-release"
			case "changed-resource":
				if err := os.WriteFile(filepath.Join(r.BundleRoot, "ardents-text-publisher.socket"), []byte("substitution"), 0600); err != nil {
					t.Fatal(err)
				}
			case "partial":
				if err := os.Remove(filepath.Join(r.BundleRoot, "ardents-text-reader.socket")); err != nil {
					t.Fatal(err)
				}
			}
			original, err := os.ReadFile(filepath.Join(r.BundleRoot, "protected-endpoint.json"))
			if err != nil {
				t.Fatal(err)
			}
			err = run(t.Context(), r.BundleRoot, release)
			if failure == "none" && err != nil || failure != "none" && err == nil {
				t.Fatalf("packaging acceptance %s: %v", failure, err)
			}
			if failure != "none" {
				code, ok := err.(*exec.ExitError)
				if !ok || code.ExitCode() != 2 {
					t.Fatalf("packaging refusal exit changed: %v", err)
				}
			}
			after, err := os.ReadFile(filepath.Join(r.BundleRoot, "protected-endpoint.json"))
			if err != nil || !bytes.Equal(original, after) {
				t.Fatal("checker rewrote committed producer bytes", err)
			}
		})
	}
}
