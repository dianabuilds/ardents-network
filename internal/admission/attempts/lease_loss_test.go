//go:build linux

package attempts

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLostRetainedLeaseCannotCreateSecondOwner(t *testing.T) {
	for _, stillLive := range []bool{true, false} {
		name := "closed owner"
		if stillLive {
			name = "live owner"
		}
		t.Run(name, func(t *testing.T) {
			first, _, record, token := tokenJournalFixture(t)
			if err := first.Mark(token, record); err != nil {
				t.Fatal(err)
			}
			if !stillLive {
				if err := first.Close(); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(first.root, "owner.lock")
			retained := filepath.Join(t.TempDir(), "retained-lock")
			before, err := os.ReadFile(filepath.Join(first.root, "attempts"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path, retained); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				second, err := Open(first.root, first.network, first.clock)
				if second != nil {
					_ = second.Close()
				}
				if err == nil || second != nil {
					t.Fatal("missing retained lease admitted another owner")
				}
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("missing lease recreated: %v", err)
				}
				after, err := os.ReadFile(filepath.Join(first.root, "attempts"))
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("refusal changed attempts: %v", err)
				}
			}
			// Exact fixture restoration only: no product recovery/reset route.
			if err := os.Rename(retained, path); err != nil {
				t.Fatal(err)
			}
			if stillLive {
				if second, err := Open(first.root, first.network, first.clock); err == nil {
					_ = second.Close()
					t.Fatal("live original lease no longer excludes another owner")
				}
				if err := first.Close(); err != nil {
					t.Fatal(err)
				}
			}
			reopened, err := Open(first.root, first.network, first.clock)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err := reopened.Mark(token, record); err == nil {
				t.Fatal("reopen forgot a retained potential spend")
			}
		})
	}
}
