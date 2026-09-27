package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0098 removed the unwired `ardents://` Service-Link formatter/parser
// from `internal/naming`; ADR-0113 then deleted the whole package tree: the
// local Stage 6 wire encoder died with its final caller (`ardents name
// encode`) and the retained alpha-only Service Link grammar plus the corpus
// and floor readers died with the ADR-0088 compatibility obligation. No
// maintained code presents, consumes, or retains any Service Link or Name
// wire grammar.
func TestNamingPackageTreeIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	if _, err := os.Stat(filepath.Join(root, "internal", "naming")); !os.IsNotExist(err) {
		t.Error("retired internal/naming package tree still exists")
	}
	profile := string(readProjectFile(t, root, "tests/profiles/deterministic-packages.txt"))
	allowlist := string(readProjectFile(t, root, "tests/profiles/deadcode-allowlist.json"))
	for _, forbidden := range []string{"internal/naming", "ParseServiceLink", "FormatServiceLink", "EncodeWire"} {
		if strings.Contains(profile, forbidden) {
			t.Errorf("deterministic package profile still lists retired naming surface %q", forbidden)
		}
		if strings.Contains(allowlist, forbidden) {
			t.Errorf("deadcode allowlist still carries retired naming surface %q", forbidden)
		}
	}
}
