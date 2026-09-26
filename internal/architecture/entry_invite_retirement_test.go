package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0106 (F-08): the Invite-root writer is retired. `entry import` and
// `entry recipient` refuse before any effect, the plan loader and every
// Invite machinery file are deleted, no working-tree code imports
// internal/entry from the operator command, and nothing can read or convert
// an existing Invite root.
func TestEntryInviteWriterIsRetired(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"cmd/ardents/entry_import.go",
		"cmd/ardents/entry_import_plan.go",
		"cmd/ardents/entry_import_test.go",
		"cmd/ardents/entry_network_fixture_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired entry import file still exists: %s", relative)
		}
	}
	retirement := string(readProjectFile(t, root, "cmd/ardents/entry_retirement.go"))
	for _, required := range []string{"errEntryCommandRetired", "case \"import\", \"recipient\":", "func entryUsageError()"} {
		if !strings.Contains(retirement, required) {
			t.Errorf("entry_retirement.go lost required refusal member %q", required)
		}
	}
	oracle := string(readProjectFile(t, root, "cmd/ardents/entry_retirement_test.go"))
	if !strings.Contains(oracle, "func TestEntryInviteCommandsRetireBeforeEffects(") {
		t.Error("entry retirement oracle is missing")
	}
	entries, err := os.ReadDir(filepath.Join(root, "cmd", "ardents"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dirEntry := range entries {
		if dirEntry.IsDir() || !strings.HasSuffix(dirEntry.Name(), ".go") {
			continue
		}
		content := string(readProjectFile(t, root, filepath.Join("cmd", "ardents", dirEntry.Name())))
		for _, forbidden := range []string{"ardents-network/internal/entry", "freshOperatorRegularFile", "loadImportPlan"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("cmd/ardents/%s regained retired entry machinery %q", dirEntry.Name(), forbidden)
			}
		}
	}
}

// The surviving internal/entry package serves the closed Entry set only. It
// is Linux-only apart from the untagged doc.go so that every platform still
// compiles the (empty) package, and the closed-set root keeps its own marker
// so a legacy Invite root can never be claimed by it.
func TestClosedEntrySetPackageIsLinuxOnly(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	doc := string(readProjectFile(t, root, "internal/entry/doc.go"))
	if strings.Contains(doc, "//go:build") {
		t.Error("doc.go must stay untagged so the package builds on every platform")
	}
	if !strings.Contains(doc, "ADR-0106") {
		t.Error("doc.go lost the Invite retirement record")
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal", "entry"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, dirEntry := range entries {
		name := dirEntry.Name()
		if dirEntry.IsDir() || !strings.HasSuffix(name, ".go") || name == "doc.go" {
			continue
		}
		found = true
		content := string(readProjectFile(t, root, filepath.Join("internal", "entry", name)))
		if !strings.HasPrefix(content, "//go:build linux") {
			t.Errorf("internal/entry/%s lost its linux-only build tag", name)
		}
	}
	if !found {
		t.Error("internal/entry lost every closed Entry set source file")
	}
	store := string(readProjectFile(t, root, "internal/entry/closed_set_store.go"))
	if !strings.Contains(store, "ardents-entry-set-v1") {
		t.Error("closed Entry set root lost its distinct marker")
	}
}
