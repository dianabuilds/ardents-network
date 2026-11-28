package plan_test

import (
	"slices"
	"testing"

	requestinput "github.com/dianabuilds/ardents-network/internal/domain/endpoint/plan"
)

func TestEndpointSnapshotStorageProjectsOnlyExplicitPublisherRoot(t *testing.T) {
	decl := endpointDeclFixture()
	const storage = "/var/lib/ardents/roles/endpoint/snapshot-storage"
	decl.Headless.SnapshotRoot = storage
	decl.Headless.SnapshotQuotaBytes = 8 << 30
	roots, err := requestinput.PlatformWritableRoots(decl)
	if err != nil || !slices.Contains(roots, storage) {
		t.Fatalf("declared snapshot storage missing from writable roots: %q, %v", roots, err)
	}

	for _, sample := range []struct {
		name   string
		change func(*requestinput.Declaration)
	}{
		{"missing quota", func(d *requestinput.Declaration) { d.Headless.SnapshotQuotaBytes = 0 }},
		{"negative quota", func(d *requestinput.Declaration) { d.Headless.SnapshotQuotaBytes = -1 }},
		{"missing root", func(d *requestinput.Declaration) { d.Headless.SnapshotRoot = "" }},
		{"reader role", func(d *requestinput.Declaration) { d.Headless.Role = "reader" }},
		{"relative root", func(d *requestinput.Declaration) { d.Headless.SnapshotRoot = "storage" }},
		{"nested in publication history", func(d *requestinput.Declaration) {
			d.Headless.SnapshotRoot = d.Headless.PublicationRoot + "/storage"
		}},
		{"encloses application socket", func(d *requestinput.Declaration) {
			d.Headless.SnapshotRoot = "/run/ardents"
		}},
		{"encloses Source credentials", func(d *requestinput.Declaration) { d.Headless.SnapshotRoot = "/etc/ardents" }},
		{"nested in immutable installation", func(d *requestinput.Declaration) {
			d.Headless.SnapshotRoot = d.InstallationRoot + "/storage"
		}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			invalid := decl
			sample.change(&invalid)
			if roots, err := requestinput.PlatformWritableRoots(invalid); err == nil {
				t.Fatalf("invalid snapshot storage produced writable roots %q", roots)
			}
		})
	}
}
