//go:build !linux && !darwin

package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
)

// The Open* compositions open the selection and Hosting durable roots, which
// are Unix-family portable (Cohort 2d, #877): Linux and macOS run the genuine
// composition. Windows has no ported Hosting measurement, so admitting it would
// create selection root state before the deeper measurement refusal; the typed
// boundary therefore refuses on Windows before opening any root, admitting any
// authority or creating any filesystem state. The exact refusal prose pins that
// the platform boundary fired, not a later input check.
func TestRouteBoundaryRefusesWithoutStateOnUnsupportedHost(t *testing.T) {
	root := t.TempDir()
	plan := funding.Plan{
		EntryRoot:    filepath.Join(root, "entry"),
		InteriorRoot: filepath.Join(root, "interior"),
		HostingRoot:  filepath.Join(root, "hosting"),
		Domain:       1,
	}
	holder := new(stock.Owner)
	ctx := context.Background()
	if _, err := OpenPrefix(ctx, plan, funding.Authority{}, holder); err == nil || err.Error() != "route requires Linux" {
		t.Fatalf("OpenPrefix platform refusal = %v, want route requires Linux", err)
	}
	if _, err := OpenBootstrap(ctx, plan, funding.Authority{}, holder); err == nil || err.Error() != "route selection and durable roots require Linux" {
		t.Fatalf("OpenBootstrap platform refusal = %v", err)
	}
	if _, err := OpenJoinContext(ctx, plan, funding.Authority{}, holder); err == nil || err.Error() != "route requires Linux" {
		t.Fatalf("OpenJoinContext platform refusal = %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unsupported-host route refusal created %d state entries", len(entries))
	}
}
