package architecture

import (
	"strings"
	"testing"
)

// ADR-0108 (F-26) contracted the Portable Endpoint profile to the two roots
// with real consumers: StateHome (owner lock, Release-floor parent,
// replacement-ledger parent) and RuntimeHome (the local attachment). The
// grants, vault, diagnostics, and cache roots and their Config fields were
// created at every Open while no non-test consumer ever used them; they must
// stay gone, and the retained roots must not lose their owner-only
// preparation.
func TestPortableProfileIsContractedToConsumedRoots(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/endpoint/portable/roots.go",
		"internal/endpoint/portable/runtime.go",
		"internal/endpoint/portable/paths_unix.go",
		"internal/endpoint/portable/paths_windows.go",
	} {
		content := string(readProjectFile(t, root, relative))
		for _, forbidden := range []string{"ConfigHome", "CacheHome", `"grants"`, `"vault"`, `"diagnostics"`, "XDG_CONFIG_HOME", "XDG_CACHE_HOME"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s still contains contracted profile scaffold %q", relative, forbidden)
			}
		}
	}
	roots := string(readProjectFile(t, root, "internal/endpoint/portable/roots.go"))
	for _, retained := range []string{
		"config.StateHome",
		"config.RuntimeHome",
		`filepath.Join(config.StateHome, "floors")`,
		`filepath.Join(config.StateHome, "live")`,
		"ensureOwnedDirectory",
	} {
		if !strings.Contains(roots, retained) {
			t.Errorf("portable roots.go lost retained consumer-root preparation %q", retained)
		}
	}
}
