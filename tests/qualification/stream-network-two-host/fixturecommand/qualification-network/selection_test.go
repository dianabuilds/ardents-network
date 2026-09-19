//go:build linux

package main

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

func TestSelectRouteInteriorIgnoresFixtureMemberOrder(t *testing.T) {
	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)
	member := func(id, subrole byte) routeMember {
		return routeMember{ClosedSetMember: entry.ClosedSetMember{
			NodeID: [32]byte{id}, PublicKey: [32]byte{id + 20}, FamilyID: [32]byte{id + 40},
			RecordDigest: [32]byte{id + 60}, DutyGeneration: uint64(id), Domain: 1, NotAfter: now.Add(time.Hour),
		}, subrole: subrole}
	}
	members := []routeMember{member(1, 1), member(2, 1), member(10, 2), member(11, 2), member(12, 2), member(13, 2)}
	firstRoot := filepath.Join(t.TempDir(), "first")
	firstEntry, firstInterior, err := selectRoute(firstRoot, [32]byte{99}, 1, members, now)
	if err != nil {
		t.Fatal(err)
	}
	secondRoot := filepath.Join(t.TempDir(), "second")
	if err := copyEntryRoot(firstRoot, secondRoot); err != nil {
		t.Fatal(err)
	}
	slices.Reverse(members)
	secondEntry, secondInterior, err := selectRoute(secondRoot, [32]byte{99}, 1, members, now)
	if err != nil {
		t.Fatal(err)
	}
	if secondEntry != firstEntry || secondInterior != firstInterior {
		t.Fatal("fixture member order changed the retained Route")
	}
}
