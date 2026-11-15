package infrastructure

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestInitialFloorsRequireEveryVersionAndDigestAbsent(t *testing.T) {
	if !emptyInitialFloors(release.FloorSet{}) {
		t.Fatal("unborn history refused")
	}
	cases := []release.FloorSet{
		{RootVersion: 1}, {TimestampVersion: 1}, {SnapshotVersion: 1}, {TargetsVersion: 1},
		{RootDigest: []byte{1}}, {TimestampDigest: []byte{1}}, {SnapshotDigest: []byte{1}}, {TargetsDigest: []byte{1}},
	}
	for i, floors := range cases {
		if emptyInitialFloors(floors) {
			t.Fatalf("retained floor %d accepted as unborn", i)
		}
	}
}
