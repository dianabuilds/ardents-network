package infrastructure_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/tests/releasefixture"
)

func TestInfrastructureInventoryPhysicalAndCanonicalRefusals(t *testing.T) {
	for _, scenario := range []string{"extra", "symlink", "hardlink", "noncanonical", "duplicate", "unknown", "invocation", "hash", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			dir, _, _ := releasefixture.InfrastructureDistribution(t, t.TempDir(), []byte("candidate bytes"), nil, 1, nil, "source")
			if _, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir); err != nil {
				t.Fatal("signed fixture refused before mutation", err)
			}
			path := filepath.Join(dir, "ardents-source.service")
			switch scenario {
			case "extra":
				if err := os.WriteFile(filepath.Join(dir, "extra"), []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("targets.json", path); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(path, filepath.Join(t.TempDir(), "linked")); err != nil {
					t.Fatal(err)
				}
			case "hash":
				if err := os.WriteFile(path, []byte("substitute"), 0600); err != nil {
					t.Fatal(err)
				}
			case "noncanonical", "duplicate", "unknown", "invocation":
				p := filepath.Join(dir, "infrastructure-enrollment.json")
				raw, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				switch scenario {
				case "noncanonical":
					raw = append(raw, '\n')
				case "duplicate":
					raw = bytes.Replace(raw, []byte("\"release_version\":1"), []byte("\"release_version\":1,\"release_version\":1"), 1)
				case "unknown":
					raw = bytes.Replace(raw, []byte("\"schema\":"), []byte("\"extra\":0,\"schema\":"), 1)
				case "invocation":
					raw = bytes.Replace(raw, []byte("\"source\""), []byte("\"issuer\""), 1)
				}
				if err := os.WriteFile(p, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx := t.Context()
			if scenario == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := enrollment.ReadInfrastructureCandidate(ctx, dir); err == nil {
				t.Fatal("invalid inventory accepted")
			}
		})
	}
}
