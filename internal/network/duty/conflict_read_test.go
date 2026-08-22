package duty

import (
	"testing"
	"time"
)

func TestReadConflictReleasesLease(t *testing.T) {
	now, root := time.Now(), t.TempDir()
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
