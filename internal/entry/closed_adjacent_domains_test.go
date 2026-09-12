//go:build linux

package entry

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClosedEntryDomainsRemainSeparateAcrossRestart(t *testing.T) {
	config, view := closedEntryFixture(t)
	originals := append([]ClosedSetMember(nil), view.Candidates...)
	for _, domain := range []uint8{3, 4} {
		for _, member := range originals {
			member.Domain = domain
			member.NodeID[1], member.PublicKey[1], member.FamilyID[1], member.RecordDigest[1] = domain, domain, domain, domain
			view.Candidates = append(view.Candidates, member)
		}
	}
	owner, err := OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	selected := make(map[uint8][2]ClosedSetMember)
	for _, domain := range []uint8{1, 3, 4} {
		pair, err := owner.Members(domain)
		if err != nil {
			_ = owner.Close()
			t.Fatalf("domain %d: %v", domain, err)
		}
		selected[domain] = pair
	}
	if _, err := owner.Members(2); err == nil {
		t.Fatal("activated non-adjacent Rendezvous")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for domain, pair := range selected {
		for slot, member := range pair {
			current, err := reopened.CurrentMember(domain, uint8(slot))
			if err != nil || current != member {
				t.Fatalf("domain %d changed selected member on reopen: %v", domain, err)
			}
		}
	}
}

func TestClosedEntryLegacyRendezvousSetRefusesWithoutReinterpretation(t *testing.T) {
	config, view := closedEntryFixture(t)
	owner, err := OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	previous := owner.name
	legacy := owner.state
	legacy.Generation, legacy.Previous = 2, previous
	set := closedDomainSet{Chosen: view.Now, NotAfter: view.Candidates[0].NotAfter}
	for i := range set.Members {
		set.Members[i] = view.Candidates[i]
		set.Members[i].Domain = 2
	}
	set.NotAfter = view.Now.Add(6 * time.Hour)
	legacy.Sets[1] = set // Exact old version-1 slot, not a new domain assignment.
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	name := sha256Hex(raw)
	path := filepath.Join(config.Root, "state-"+name)
	if err := writeGeneration(config.Root, path, raw); err != nil {
		t.Fatal(err)
	}
	if err := replaceWatermark(config.Root, legacy.Generation, name); err != nil {
		t.Fatal(err)
	}
	if err := replaceCurrent(config.Root, name); err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenClosedSets(config); err == nil {
		_ = reopened.Close()
		t.Fatal("reopened forbidden old Rendezvous adjacency")
	}
	retained, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, retained) {
		t.Fatal("refusal changed old retained selection")
	}
}
