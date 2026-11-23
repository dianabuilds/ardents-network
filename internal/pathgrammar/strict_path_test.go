package pathgrammar

import (
	"path/filepath"
	"testing"
)

// The shared predicate must be exactly the formerly repeated inline grammar
// on every host platform: absolute and already clean, with no conversion.
func TestStrictMatchesTheFormerInlineGrammar(t *testing.T) {
	corpus := []string{
		"", "relative", "relative/child", "..", "../escape", ".", "./child",
		"/", "/clean/absolute", "/trailing/", "/dot/./segment", "/dot/../segment",
		"/double//separator", "/nul\x00rune", "C:\\", "C:\\clean", "C:\\trailing\\",
		"\\\\host\\share", string(filepath.Separator),
	}
	for _, path := range corpus {
		inline := filepath.IsAbs(path) && filepath.Clean(path) == path
		if Strict(path) != inline {
			t.Errorf("Strict(%q) = %v, former inline grammar = %v", path, Strict(path), inline)
		}
	}
}

func TestStrictAdmitsOnlyAbsoluteCleanHostPaths(t *testing.T) {
	base := t.TempDir()
	separator := string(filepath.Separator)
	accepted := []string{
		base,
		filepath.Join(base, "child"),
		filepath.Join(base, "nested", "leaf"),
	}
	for _, path := range accepted {
		if !Strict(path) {
			t.Errorf("absolute clean path refused: %q", path)
		}
	}
	refused := []string{
		"",
		"relative",
		"relative/child",
		base + separator,
		base + separator + ".",
		base + separator + "child" + separator,
		base + separator + "double" + separator + separator + "child",
		base + separator + "dot" + separator + "." + separator + "child",
		base + separator + "dot" + separator + ".." + separator + "child",
	}
	for _, path := range refused {
		if Strict(path) {
			t.Errorf("non-absolute or non-clean path admitted: %q", path)
		}
	}
}

// The predicate reports; it never absolutizes a relative path the way the
// former Network State configuration validation did.
func TestStrictNeverConvertsRelativePaths(t *testing.T) {
	for _, path := range []string{"relative", "relative/child", "./child", "../sibling"} {
		if Strict(path) {
			t.Errorf("relative path admitted without conversion: %q", path)
		}
	}
}
