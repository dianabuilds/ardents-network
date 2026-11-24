package command

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReadRolePlanPreservesFlagsAndInputBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan")
	limit := bytes.Repeat([]byte("x"), 16<<10)
	if err := os.WriteFile(path, limit, 0600); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--config", "--retained-config"} {
		raw, retained, err := ReadRolePlan([]string{flag, path})
		if err != nil || retained != (flag == "--retained-config") || !bytes.Equal(raw, limit) {
			t.Fatalf("%s changed the declaration or retained mode: retained=%v err=%v", flag, retained, err)
		}
	}
	if err := os.WriteFile(path, append(limit, 'x'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadRolePlan([]string{"--config", path}); err == nil {
		t.Fatal("oversized declaration accepted")
	}
	if _, _, err := ReadRolePlan([]string{"--unknown", path}); err == nil {
		t.Fatal("unknown declaration flag accepted")
	}
}
