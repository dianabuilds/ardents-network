//go:build linux

package selection

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

func TestInteriorReopenRetainsPairDeadlineAndFloor(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	member := func(id byte) route.Member {
		return route.Member{NodeID: [32]byte{id}, PublicKey: [32]byte{id + 10}, FamilyID: [32]byte{id + 20}, RecordDigest: [32]byte{id + 30}, DutyGeneration: 1, Domain: 1, NotAfter: now.Add(time.Hour)}
	}
	entries := [2]route.Member{member(1), member(2)}
	candidates := []route.Member{member(3), member(4), member(5)}
	root := filepath.Join(t.TempDir(), "interior")
	owner, err := openInteriorStore(root, [32]byte{1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	first, err := owner.selectPair(candidates, entries, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.close(); err != nil {
		t.Fatal(err)
	}
	owner, err = openInteriorStore(root, [32]byte{1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	second, err := owner.selectPair(candidates, entries, now.Add(time.Minute))
	if err != nil || second != first {
		t.Fatal("reopen renewed or replaced pair", err)
	}
	if _, err := owner.selectPair(candidates, entries, now.Add(-time.Second)); err == nil {
		t.Fatal("reopen reset time floor")
	}
	if _, err := owner.selectPair([]route.Member{first.Members[1]}, entries, now.Add(time.Minute)); err == nil {
		t.Fatal("unavailable member caused redraw")
	}
	before, err := os.ReadFile(filepath.Join(root, "watermark"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = owner.selectPair(candidates, [2]route.Member{member(6), member(7)}, now.Add(time.Minute))
	after, err := os.ReadFile(filepath.Join(root, "watermark"))
	if err != nil || string(after) != string(before) {
		t.Fatal("refusal changed durable generation", err)
	}
}
