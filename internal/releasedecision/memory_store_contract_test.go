package releasedecision

import (
	"bytes"
	"errors"
	"testing"
)

func closeStoreForTest(t *testing.T, store Store) {
	t.Helper()
	if err := store.Close(); err != nil {
		t.Errorf("close store: %v", err)
	}
}

func validateMemoryFloorSet(floors FloorSet) error {
	if floors.RootVersion <= 0 || len(floors.RootDigest) != 32 {
		return errors.New("test store: incomplete root floor")
	}
	metadataEmpty := floors.TimestampVersion == 0 && len(floors.TimestampDigest) == 0 &&
		floors.SnapshotVersion == 0 && len(floors.SnapshotDigest) == 0 &&
		floors.TargetsVersion == 0 && len(floors.TargetsDigest) == 0
	if metadataEmpty {
		return nil
	}
	if floors.TimestampVersion <= 0 || len(floors.TimestampDigest) != 32 ||
		floors.SnapshotVersion <= 0 || len(floors.SnapshotDigest) != 32 ||
		floors.TargetsVersion <= 0 || len(floors.TargetsDigest) != 32 {
		return errors.New("test store: incomplete metadata floors")
	}
	return nil
}

func validateMemoryAdvance(previous, next FloorSet) error {
	roles := []struct {
		previousVersion int64
		nextVersion     int64
		previousDigest  []byte
		nextDigest      []byte
	}{
		{previous.RootVersion, next.RootVersion, previous.RootDigest, next.RootDigest},
		{previous.TimestampVersion, next.TimestampVersion, previous.TimestampDigest, next.TimestampDigest},
		{previous.SnapshotVersion, next.SnapshotVersion, previous.SnapshotDigest, next.SnapshotDigest},
		{previous.TargetsVersion, next.TargetsVersion, previous.TargetsDigest, next.TargetsDigest},
	}
	for _, role := range roles {
		if role.nextVersion < role.previousVersion {
			return errors.New("test store: floor decreased")
		}
		if role.nextVersion == role.previousVersion && role.previousVersion != 0 &&
			!bytes.Equal(role.nextDigest, role.previousDigest) {
			return errors.New("test store: digest changed at the same version")
		}
	}
	return nil
}
