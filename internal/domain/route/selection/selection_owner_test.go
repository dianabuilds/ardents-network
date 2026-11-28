package selection

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

// The standalone owner composes the same shared Entry and Interior stores as
// the installation borrowers. These supplied-fact openings establish no
// authenticated intake or successful Route transport evidence.
func TestOwnerOpenRetainsChoiceAndReleasesPrivateInstallation(t *testing.T) {
	root := ownedRootFixture(t)
	view := installationView(t, 1)
	config := Config{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 3, Current: func() (network.RuntimeView, error) { return view, nil }}
	owner, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	leg, err := owner.Select()
	if err != nil {
		t.Fatal(err)
	}
	if leg.EntryMember.RoleDomain != 3 {
		t.Fatal("owner lost role binding")
	}
	repeated, err := owner.Select()
	if err != nil || repeated.Entry != leg.Entry || repeated.Interior != leg.Interior || repeated.NotAfter != leg.NotAfter {
		t.Fatal("second select redrew the retained choice", err)
	}
	if second, err := Open(config); err == nil {
		_ = second.Close()
		t.Fatal("two owners claimed the same Entry root")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Select(); err == nil {
		t.Fatal("selection after close")
	}
	reopened, err := Open(config)
	if err != nil {
		t.Fatal("Close did not release the private Entry installation", err)
	}
	retained, err := reopened.Select()
	if err != nil || retained.Entry != leg.Entry || retained.Interior != leg.Interior || retained.NotAfter != leg.NotAfter {
		t.Fatal("reopen changed the retained selection", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerOpenRefusesOverlapAndIncompleteSetupWithoutCreation(t *testing.T) {
	root := ownedRootFixture(t)
	view := installationView(t, 1)
	current := func() (network.RuntimeView, error) { return view, nil }
	entry := filepath.Join(root, "entry")
	for _, config := range []Config{
		{EntryRoot: entry, InteriorRoot: entry, Domain: 3, Current: current},
		{EntryRoot: entry, InteriorRoot: filepath.Join(entry, "nested"), Domain: 3, Current: current},
		{EntryRoot: root, InteriorRoot: filepath.Join(root, "interior"), Domain: 3, Current: current},
		{EntryRoot: entry, InteriorRoot: filepath.Join(root, "interior"), Domain: 2, Current: current},
		{EntryRoot: entry, InteriorRoot: filepath.Join(root, "interior"), Domain: 3},
		{InteriorRoot: filepath.Join(root, "interior"), Domain: 3, Current: current},
	} {
		if owner, err := Open(config); err == nil {
			_ = owner.Close()
			t.Fatalf("invalid standalone setup accepted: %+v", config)
		}
	}
	for _, path := range []string{entry, filepath.Join(entry, "nested"), filepath.Join(root, "interior")} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("refused opening created root %s: %v", path, err)
		}
	}
}
