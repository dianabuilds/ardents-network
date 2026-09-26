package architecture

import (
	"strings"
	"testing"
)

// ADR-0098 removed the unwired `ardents://` Service-Link formatter/parser
// from `internal/naming`: no accepted C0 command presents or consumes a
// Service Link there, and the retained alpha-only Service Link grammar
// lives separately in `internal/naming/alpha` under ADR-0088.
func TestNamingServiceLinkTracerIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	names := string(readProjectFile(t, root, "internal/naming/name.go"))
	for _, forbidden := range []string{"ParseServiceLink", "FormatServiceLink", "serviceLinkScheme", "ardents://"} {
		if strings.Contains(names, forbidden) {
			t.Errorf("name.go still contains rejected Service-Link surface %q", forbidden)
		}
	}
	for _, retained := range []string{"func Parse(raw string) (Name, error)", "func IsDescendant(child, parent Name) bool", "func parseName(raw string) (Name, error)"} {
		if !strings.Contains(names, retained) {
			t.Errorf("name.go lost retained declaration %q", retained)
		}
	}
	if !strings.Contains(names, "must not contain URL scheme") {
		t.Error("name.go lost the unconditional scheme rejection in parseName")
	}
	alpha := string(readProjectFile(t, root, "internal/naming/alpha/service_link.go"))
	if !strings.Contains(alpha, "func ParseServiceLink(raw string) (ServiceLink, error)") {
		t.Error("alpha-only Service Link grammar (ADR-0088 retained evidence) was disturbed")
	}
}
