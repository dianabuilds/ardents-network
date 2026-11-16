package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The legacy generic Node runtime, issuer and hosting operator paths are retired
// before any private configuration is read or any root effect is attempted. Each
// maintained operation uses its explicit successor composition through the
// canonical ardents-next commands; these refusals grant no runtime or adoption.
func TestLegacyNodeCommandPathsRefuseBeforeEffects(t *testing.T) {
	for _, test := range []struct {
		name      string
		arguments []string
		refusal   error
	}{
		{name: "node config", arguments: []string{"node", "--config", "missing.json"}, refusal: errOldNodeConfigRetired},
		{name: "issuer initialize", arguments: []string{"issuer", "initialize", "--config", "missing.json"}, refusal: errOldIssuerRetired},
		{name: "issuer serve", arguments: []string{"issuer", "serve", "--config", "missing.json"}, refusal: errOldIssuerRetired},
		{name: "hosting initialize", arguments: []string{"hosting", "initialize", "--config", "missing.json"}, refusal: errOldHostingRetired},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			socket := filepath.Join(root, "diagnostics.socket")
			t.Setenv("ARDENTS_DEBUG_SOCKET", socket)
			var output bytes.Buffer
			err := runCommand(context.Background(), test.arguments, &output)
			if !errors.Is(err, test.refusal) {
				t.Fatalf("legacy path error = %v; want %v", err, test.refusal)
			}
			if output.Len() != 0 {
				t.Fatalf("retired path wrote output: %s", output.String())
			}
			if _, err := os.Lstat(socket); !os.IsNotExist(err) {
				t.Fatalf("retired path opened diagnostic socket: %v", err)
			}
		})
	}
}
