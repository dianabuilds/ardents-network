//go:build !linux && !darwin

package publisher

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
)

// Live Open composes the Unix-family selection, Hosting, Instance custody and
// durable history roots with the portable domain Publisher (Linux and macOS,
// #883); it launches no native worker itself. Windows lacks the ported Hosting
// measurement and Instance custody, so the typed boundary must refuse before
// opening any root or creating any filesystem state.
func TestOpenBoundaryRefusesWithoutStateOnUnsupportedHost(t *testing.T) {
	root := t.TempDir()
	plan := Plan{InstanceRoot: filepath.Join(root, "instance"), Root: filepath.Join(root, "publication")}
	routePlan := funding.Plan{
		EntryRoot:             filepath.Join(root, "entry"),
		InteriorRoot:          filepath.Join(root, "interior"),
		SourceInteriorRoot:    filepath.Join(root, "source-interior"),
		ResponderInteriorRoot: filepath.Join(root, "responder-interior"),
		HostingRoot:           filepath.Join(root, "hosting"),
		Domain:                4,
	}
	_, err := Open(context.Background(), plan, routePlan, funding.Authority{}, new(stock.Owner), new(executionruntime.Operation))
	if err == nil || err.Error() != "live Publisher native ownership unavailable" {
		t.Fatalf("Open platform refusal = %v, want live Publisher native ownership unavailable", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unsupported-host Open refusal created %d state entries", len(entries))
	}
}
