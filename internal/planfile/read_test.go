package planfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func TestReadAndDecodeEnforceBoundsAndShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(`{"name":"node"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var value struct {
		Name string `json:"name"`
	}
	if err := planfile.Decode(path, 32, &value); err != nil || value.Name != "node" {
		t.Fatalf("decode = %+v, %v", value, err)
	}
	if _, err := planfile.Read(path, 4); err == nil {
		t.Fatal("oversized plan was accepted")
	}
}
