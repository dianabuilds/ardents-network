//go:build windows

package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNormalizeWindowsFinalPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: `\\?\C:\ProgramData\Ardents\update\.ardents-update-transaction-lock`, want: `C:\ProgramData\Ardents\update\.ardents-update-transaction-lock`},
		{path: `\\?\UNC\server\share\update\.ardents-update-transaction-lock`, want: `\\server\share\update\.ardents-update-transaction-lock`},
		{path: `C:\ProgramData\Ardents\update\.ardents-update-transaction-lock`, want: ""},
	}
	for _, test := range tests {
		if got := normalizeWindowsFinalPath(test.path); got != test.want {
			t.Errorf("normalizeWindowsFinalPath(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

// TestRecoverRejectsLockPathAlias proves through the public recovery boundary
// that the locked handle must resolve to the admitted lock pathname. A Windows
// directory junction has a distinct textual root while resolving to the same
// physical root, so it exercises a real held-handle/path mismatch without
// requiring symbolic-link privilege.
func TestRecoverRejectsLockPathAlias(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	alias := filepath.Join(base, "alias")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	oracleBootstrapV0(t, target)
	command := exec.Command("cmd", "/c", "mklink", "/J", alias, target)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create directory junction: %v: %s", err, output)
	}
	t.Cleanup(func() {
		if output, err := exec.Command("cmd", "/c", "rmdir", alias).CombinedOutput(); err != nil {
			t.Errorf("remove directory junction: %v: %s", err, output)
		}
	})

	result, err := Recover(context.Background(), alias)
	recoveryOracleAssertInvalid(t, result, err)
}
