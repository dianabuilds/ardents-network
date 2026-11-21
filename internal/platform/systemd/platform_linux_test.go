package systemd

import (
	"strings"
	"testing"
)

// This is only the finite observation vocabulary, not a fabricated accepting
// manager. Actual typed absence, original transport and Close remain native.
func TestPlatformSiblingVocabularyPreservesFixedNodeExclusion(t *testing.T) {
	for _, role := range []string{"forwarding", "issuer", "descriptor", "introduction", "join"} {
		names, err := platformSiblingNames(role)
		if err != nil || len(names) != 9 {
			t.Fatal(role, names, err)
		}
		seen := map[string]bool{}
		for _, name := range names {
			if seen[name] || name == "ardents-"+role+".service" || strings.Contains(name, "source") {
				t.Fatal("foreign or duplicate slot", role, name)
			}
			seen[name] = true
		}
		if !seen["ardents-"+role+".socket"] {
			t.Fatal("own activation socket escaped exclusion")
		}
	}
	for _, role := range []string{"endpoint", "other", "forwarding.service", "../forwarding"} {
		if _, err := platformSiblingNames(role); err == nil {
			t.Fatal("arbitrary unit selector admitted", role)
		}
	}
}
