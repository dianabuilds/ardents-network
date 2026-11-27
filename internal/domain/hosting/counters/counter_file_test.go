package counters

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// boundedFile is the portable bounded read shared by the native counter
// adapter. Its limit and joined-error semantics are platform-independent, so
// they execute on every mandatory target even though the genuine production
// caller (Read) stays Linux.
func TestBoundedFileEnforcesLimitAndJoinedErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "counter")
	if err := os.WriteFile(path, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, maximum := range []int64{5, 6, 64} {
		body, err := boundedFile(path, maximum)
		if err != nil || string(body) != "12345" {
			t.Fatalf("bounded read within limit %d: %q %v", maximum, body, err)
		}
	}
	body, err := boundedFile(path, 4)
	if body != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("over-limit read must refuse with ErrUnavailable: %q %v", body, err)
	}
	if _, err := boundedFile(filepath.Join(dir, "absent"), 64); err == nil {
		t.Fatal("missing counter file must refuse")
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if body, err := boundedFile(empty, 0); err != nil || len(body) != 0 {
		t.Fatalf("empty bounded read: %q %v", body, err)
	}
}
