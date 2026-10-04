package duty

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadConflictReleasesLease(t *testing.T) {
	now, root := time.Now(), localRoleFixtureRoot(t)
	roles, err := Open(Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	if conflict, err := ReadConflict(root, func() time.Time { return now }, [32]byte{1}, [32]byte{2}); err != nil || conflict {
		t.Fatalf("one-shot conflict = %v, %v", conflict, err)
	}
	reopened, err := Open(Config{Root: root, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

// localRoleFixtureRoot creates the owner-only local-role root required by the
// Unix permission contract, independent of the test process umask.
func localRoleFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "local-roles")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
