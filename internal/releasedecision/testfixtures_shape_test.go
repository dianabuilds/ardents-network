package releasedecision

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func withTargetCount(t *testing.T, repo syntheticRepository, count int) syntheticRepository {
	t.Helper()
	targetBytes, err := repo.targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := metadata.Targets().FromBytes(targetBytes)
	if err != nil {
		t.Fatal(err)
	}
	template := targets.Signed.Targets[repo.targetPath]
	for index := 1; index < count; index++ {
		name := fmt.Sprintf("ardents/windows-amd64/spare-%04d", index)
		custom := json.RawMessage(append([]byte(nil), (*template.Custom)...))
		targets.Signed.Targets[name] = &metadata.TargetFiles{
			Length: template.Length, Hashes: template.Hashes, Path: name, Custom: &custom,
		}
	}
	targets.Signatures = nil
	signSyntheticMetadataCount(t, targets, repo.keys, ordinaryThreshold)
	targetBytes, err = targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	repo.files["https://release.invalid/metadata/1.targets.json"] = targetBytes
	targetDigest := sha256.Sum256(targetBytes)

	snapshotBytes, err := repo.snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := metadata.Snapshot().FromBytes(snapshotBytes)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(targetBytes)), Hashes: metadata.Hashes{"sha256": targetDigest[:]}}
	snapshot.Signatures = nil
	signSyntheticMetadataCount(t, snapshot, repo.keys, ordinaryThreshold)
	snapshotBytes, err = snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	repo.files["https://release.invalid/metadata/1.snapshot.json"] = snapshotBytes
	snapshotDigest := sha256.Sum256(snapshotBytes)

	timestampBytes, err := repo.timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	timestamp, err := metadata.Timestamp().FromBytes(timestampBytes)
	if err != nil {
		t.Fatal(err)
	}
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	timestamp.Signatures = nil
	signSyntheticMetadataCount(t, timestamp, repo.keys, ordinaryThreshold)
	timestampBytes, err = timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	repo.files["https://release.invalid/metadata/timestamp.json"] = timestampBytes
	repo.targets, repo.snapshot, repo.timestamp = targets, snapshot, timestamp
	return repo
}
