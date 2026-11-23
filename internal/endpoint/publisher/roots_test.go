package publisher

import (
	"path/filepath"
	"testing"
)

// The root-isolation predicate is pure path grammar: it composes only the
// shared strict path predicate and host separator, so its behavior is exercised
// on every mandatory target rather than only beside the Linux composition.
func TestIndependentRoots(t *testing.T) {
	base := t.TempDir()
	a := filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	child := filepath.Join(a, "child")
	textSibling := filepath.Join(base, "ab")
	for _, test := range []struct {
		name  string
		roots []string
		want  bool
	}{
		{"distinct siblings", []string{a, b}, true},
		{"no roots", nil, true},
		{"equal roots", []string{a, a}, false},
		{"parent then child", []string{a, child}, false},
		{"child then parent", []string{child, a}, false},
		{"shared text prefix is not a path prefix", []string{a, textSibling}, true},
		{"relative root refused", []string{"relative"}, false},
		{"non-clean root refused", []string{a + string(filepath.Separator)}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := independentRoots(test.roots...); got != test.want {
				t.Fatalf("independentRoots(%q) = %v, want %v", test.roots, got, test.want)
			}
		})
	}
}
