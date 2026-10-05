//go:build linux

package selection

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// These are supplied Network model facts for selection component tests, not
// authenticated intake or successful Route transport evidence.
func installationView(t *testing.T, networkID byte) network.RuntimeView {
	t.Helper()
	now := time.Unix(1900000000, 0).UTC()
	p := network.ProfileBinding{Network: [32]byte{networkID}, Generation: [32]byte{2}, EpochDigest: [32]byte{3}, Digest: [32]byte{4}, Epoch: 1, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
	var assignments []network.Assignment
	var records []network.NodeRecord
	for domain := uint8(3); domain <= 4; domain++ {
		for subrole := uint8(1); subrole <= 2; subrole++ {
			for n := 0; n < 2; n++ {
				id := byte(len(records) + 1)
				name := "responder"
				if domain == 4 {
					name = "introduction"
				}
				r := network.NodeRecord{NodeID: [32]byte{id}, RecordDigest: [32]byte{id + 20}, PublicKey: [32]byte{id + 40}, FamilyID: [32]byte{id + 60}, Generation: 1, Capacity: 1, Assignment: name, CarrierProfile: "ardents-carrier-tcp-tls-v2", ValidFrom: p.NotBefore, ValidUntil: p.NotAfter, AssignmentNotAfter: p.NotAfter}
				records = append(records, r)
				assignments = append(assignments, network.Assignment{NodeID: r.NodeID, RecordDigest: r.RecordDigest, Generation: 1, RoleDomain: domain, Subrole: subrole})
			}
		}
	}
	membership, err := network.BindMembership(p, assignments, records)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := network.BindAcceptedState(network.EpochFacts{Network: p.Network, Number: p.Epoch, Digest: p.EpochDigest, ValidFrom: p.NotBefore, ValidUntil: p.NotAfter}, p.Generation, network.ProfileFacts{ProfileBinding: p, IssuanceAuthorityKey: [32]byte{7}, IssuerNodeID: records[0].NodeID, IssuerDutyGeneration: 1, TokenKeys: []network.TokenKey{{WindowStart: now, Class: 1, SPKI: [346]byte{1}}}}, membership)
	if err != nil {
		t.Fatal(err)
	}
	clock, err := network.ConfirmTime(network.ClockEvidence{Wall: now, Independent: now}, 0)
	if err != nil {
		t.Fatal(err)
	}
	view, err := accepted.Observe(clock)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestInstallationSealWaitsForBorrowers(t *testing.T) {
	root := t.TempDir()
	view := installationView(t, 1)
	config := InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: func() (network.RuntimeView, error) { return view, nil }}
	installation, err := OpenInstallation(config)
	if err != nil {
		t.Fatal(err)
	}
	borrower, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "interior"), Domain: 3})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- installation.Close() }()
	deadline := time.Now().Add(time.Second)
	for installation.available() == nil {
		if time.Now().After(deadline) {
			t.Fatal("Close did not seal")
		}
		runtime.Gosched()
	}
	if _, err := borrower.Select(); err == nil {
		t.Fatal("selection after seal")
	}
	if _, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "late"), Domain: 4}); err == nil {
		t.Fatal("borrow after seal")
	}
	if _, err := os.Lstat(filepath.Join(root, "late")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("denied borrow created root: %v", err)
	}
	select {
	case err := <-done:
		t.Fatalf("close returned with outstanding borrower: %v", err)
	default:
	}
	if _, err := OpenInstallation(config); err == nil {
		t.Fatal("Entry lease released before borrower")
	}
	if err := borrower.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not join")
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenInstallation(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationRootAliasesAndDescendantsRefused(t *testing.T) {
	root := t.TempDir()
	view := installationView(t, 1)
	entry := filepath.Join(root, "entry")
	installation, err := OpenInstallation(InstallationConfig{EntryRoot: entry, Current: func() (network.RuntimeView, error) { return view, nil }})
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(entry, alias); err != nil {
		t.Fatal(err)
	}
	for _, interior := range []string{entry, filepath.Join(entry, "nested"), root, alias, filepath.Join(alias, "nested")} {
		if owner, err := installation.Borrow(RoleConfig{InteriorRoot: interior, Domain: 3}); err == nil {
			owner.Close()
			t.Fatalf("overlapping root accepted: %s", interior)
		}
	}
	if _, err := os.Lstat(filepath.Join(entry, "nested")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refusal mutated Entry root: %v", err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationLateBorrowCurrentnessAndSeal(t *testing.T) {
	for _, loss := range []string{"Network", "seal"} {
		t.Run(loss, func(t *testing.T) {
			root := t.TempDir()
			first := installationView(t, 1)
			other := installationView(t, 2)
			var armed atomic.Bool
			var observations atomic.Int32
			reached := make(chan struct{})
			resume := make(chan struct{})
			installation, err := OpenInstallation(InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: func() (network.RuntimeView, error) {
				if armed.Load() && observations.Add(1) == 2 {
					close(reached)
					<-resume
					if loss == "Network" {
						return other, nil
					}
				}
				return first, nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			armed.Store(true)
			type outcome struct {
				owner *Owner
				err   error
			}
			result := make(chan outcome, 1)
			go func() {
				owner, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "interior"), Domain: 3})
				result <- outcome{owner, err}
			}()
			select {
			case <-reached:
			case <-time.After(time.Second):
				t.Fatal("Borrow did not reach post-root observation")
			}
			closed := make(chan error, 1)
			if loss == "seal" {
				go func() { closed <- installation.Close() }()
				deadline := time.Now().Add(time.Second)
				for installation.available() == nil {
					if time.Now().After(deadline) {
						t.Fatal("Close did not seal opening")
					}
					runtime.Gosched()
				}
				select {
				case <-closed:
					t.Fatal("Close returned before opening joined")
				default:
				}
			}
			close(resume)
			select {
			case got := <-result:
				if got.err == nil || got.owner != nil {
					if got.owner != nil {
						got.owner.Close()
					}
					t.Fatal("late Borrow accepted")
				}
			case <-time.After(time.Second):
				t.Fatal("Borrow did not return")
			}
			if loss == "seal" {
				if err := <-closed; err != nil {
					t.Fatal(err)
				}
			} else {
				if err := installation.Close(); err != nil {
					t.Fatal(err)
				}
			}
			// The refused opening returned its own Interior lease; retained bytes remain
			// usable on a subsequent genuine model observation of the original Network.
			reopened, err := OpenInstallation(InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: func() (network.RuntimeView, error) { return first, nil }})
			if err != nil {
				t.Fatal(err)
			}
			borrower, err := reopened.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "interior"), Domain: 3})
			if err != nil {
				t.Fatal(err)
			}
			if err := borrower.Close(); err != nil {
				t.Fatal(err)
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInstallationRoleExclusionCannotRedrawSharedEntry(t *testing.T) {
	root := t.TempDir()
	view := installationView(t, 1)
	installation, err := OpenInstallation(InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: func() (network.RuntimeView, error) { return view, nil }})
	if err != nil {
		t.Fatal(err)
	}
	first, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "first"), Domain: 3})
	if err != nil {
		t.Fatal(err)
	}
	leg, err := first.Select()
	if err != nil {
		t.Fatal(err)
	}
	excluded := route.Member{NodeID: leg.EntryMember.NodeID, PublicKey: leg.EntryMember.PublicKey, FamilyID: leg.EntryMember.FamilyID}
	second, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "second"), Domain: 3, Exclusions: []route.Member{excluded}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Select(); err == nil {
		t.Fatal("role exclusion ignored")
	}
	retained, err := first.Select()
	if err != nil || retained.Entry != leg.Entry || retained.Interior != leg.Interior {
		t.Fatalf("role changed shared retained choice: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationSharedEntrySeparateRolesRetainedReopen(t *testing.T) {
	root := t.TempDir()
	view := installationView(t, 1)
	config := InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: func() (network.RuntimeView, error) { return view, nil }}
	installation, err := OpenInstallation(config)
	if err != nil {
		t.Fatal(err)
	}
	a, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "a"), Domain: 3})
	if err != nil {
		t.Fatal(err)
	}
	b, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "b"), Domain: 4})
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.Select()
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.Select()
	if err != nil {
		t.Fatal(err)
	}
	if first.EntryMember.RoleDomain != 3 || second.EntryMember.RoleDomain != 4 {
		t.Fatal("role binding lost")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	retained, err := b.Select()
	if err != nil || retained.Entry != second.Entry || retained.Interior != second.Interior || retained.NotAfter != second.NotAfter {
		t.Fatalf("sibling invalidated or redrawn: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
	installation, err = OpenInstallation(config)
	if err != nil {
		t.Fatal(err)
	}
	a, err = installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "a"), Domain: 3})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := a.Select()
	if err != nil || reopened.Entry != first.Entry || reopened.Interior != first.Interior || reopened.NotAfter != first.NotAfter {
		t.Fatalf("reopen changed retained selection: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationRetainsBorrowerCleanupFailure(t *testing.T) {
	root := t.TempDir()
	view := installationView(t, 1)
	installation, err := OpenInstallation(InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: func() (network.RuntimeView, error) { return view, nil }})
	if err != nil {
		t.Fatal(err)
	}
	borrower, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "interior"), Domain: 3})
	if err != nil {
		t.Fatal(err)
	}
	// A physically broken lease exercises cleanup provenance; it supplies no
	// accepting authority or transport behavior.
	if err := borrower.interior.lease.file.Close(); err != nil {
		t.Fatal(err)
	}
	terminal := borrower.Close()
	if terminal == nil {
		t.Fatal("lost Interior physical cleanup failure")
	}
	if repeated := borrower.Close(); repeated != terminal {
		t.Fatal("borrower terminal result changed")
	}
	installationTerminal := installation.Close()
	if installationTerminal == nil {
		t.Fatal("installation lost returned borrower cleanup failure")
	}
	if !errors.Is(installationTerminal, terminal) {
		t.Fatalf("installation lost cleanup provenance: %v", installationTerminal)
	}
	if repeated := installation.Close(); repeated != installationTerminal {
		t.Fatal("installation terminal result changed")
	}
}

func TestInstallationRetainedLegOwnsAllKnownExclusions(t *testing.T) {
	root := t.TempDir()
	view := installationView(t, 1)
	installationKnown := []route.Member{{NodeID: [32]byte{250}, PublicKey: [32]byte{250}, FamilyID: [32]byte{250}}}
	roleKnown := []route.Member{{NodeID: [32]byte{251}, PublicKey: [32]byte{251}, FamilyID: [32]byte{251}}}
	expectedInstallation, expectedRole := installationKnown[0], roleKnown[0]
	installation, err := OpenInstallation(InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: func() (network.RuntimeView, error) { return view, nil }, Exclusions: installationKnown})
	if err != nil {
		t.Fatal(err)
	}
	borrower, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "interior"), Domain: 3, Exclusions: roleKnown})
	if err != nil {
		t.Fatal(err)
	}
	installationKnown[0] = route.Member{}
	roleKnown[0] = route.Member{}
	leg, err := borrower.Select()
	if err != nil {
		t.Fatal(err)
	}
	foundInstallation, foundRole := false, false
	for _, known := range leg.known {
		foundInstallation = foundInstallation || known == expectedInstallation
		foundRole = foundRole || known == expectedRole
	}
	if !foundInstallation || !foundRole {
		t.Fatal("retained leg forgot copied installation/role exclusions")
	}
	if err := borrower.Close(); err != nil {
		t.Fatal(err)
	}
	if err := installation.Close(); err != nil {
		t.Fatal(err)
	}
}
