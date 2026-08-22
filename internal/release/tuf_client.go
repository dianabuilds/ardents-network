package release

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
)

// targetRole is the constant go-tuf uses for the top-level targets
// role. It is repeated here so the package does not need to import
// the go-tuf metadata package solely for the constant.
const targetRole = "targets"

// metadataBaseURL is the only https base URL the offline envelope
// uses. It is fixed so two distributors returning identical bytes look
// like the same metadata server to go-tuf.
const metadataBaseURL = "https://release.invalid/metadata/"

// verifiedSet wraps the TUF trusted-metadata set with the ardents
// floors, the RefTime, and the bounded envelope.
type verifiedSet struct {
	set            trustedmetadata.TrustedMetadata
	envelope       envelopeState
	rootBytes      []byte
	rootChain      []rootPublication
	timestampBytes []byte
	snapshotBytes  []byte
	targetsBytes   []byte
}

// rootPublication is one verified root's bytes plus its version and
// digest for the durable floor.
type rootPublication struct {
	Version int64
	Digest  []byte
	Bytes   []byte
}

// buildVerifiedSet runs the top-level go-tuf trusted-metadata workflow
// against the bounded byte Adapter and returns the exact verified bytes.
func buildVerifiedSet(ctx context.Context, in Inputs, refTime time.Time, store floorPersistence, durable FloorSet) (*verifiedSet, Decision) {
	if err := ctx.Err(); err != nil {
		return nil, reject(outcomeReleaseUnavailable, "evaluation context is cancelled", err)
	}
	if err := validateInputsEnvelope(in); err != nil {
		return nil, reject(outcomeReleaseInvalid, err.Error(), err)
	}
	if len(in.RootBytes) == 0 {
		return nil, reject(outcomeReleaseInvalid, "trusted root is missing", nil)
	}
	if _, err := metadataRootFromBytes(in.RootBytes); err != nil {
		return nil, reject(outcomeReleaseInvalid, "trusted root is malformed", err)
	}
	fetcher, err := newMapFetcher(metadataBaseURL, in.Files)
	if err != nil {
		return nil, reject(outcomeReleaseInvalid, err.Error(), err)
	}
	trusted, err := trustedmetadata.New(in.RootBytes)
	if err != nil {
		return nil, reject(outcomeReleaseInvalid, "trusted metadata construction failed", err)
	}
	trusted.RefTime = refTime
	policy, err := validateRootPolicy(trusted.Root, in.Local, refTime, nil)
	if err != nil {
		return nil, reject(classifyReleaseError(err), detailInvalidMessage(err), err)
	}
	chain := []rootPublication{newRootPublication(trusted.Root.Signed.Version, in.RootBytes)}
	initial, rotationErr := checkRootRotation(chain, durable)
	if rotationErr != nil {
		outcome := outcomeReleaseInvalid
		if initial.conflict {
			outcome = outcomeReleaseConflict
		}
		return nil, reject(outcome, rotationErr.Error(), rotationErr)
	}
	if durable.RootVersion == 0 {
		if err := store.CommitRoot(chain[0].Version, chain[0].Digest, rootBytes(chain)); err != nil {
			return nil, reject(outcomeReleaseInvalid, "publish initial trusted root failed", err)
		}
	}
	for rotations := int64(0); rotations < maximumRootRotations; rotations++ {
		nextVersion := trusted.Root.Signed.Version + 1
		data, fetchErr := fetcher.DownloadFile(metadataURL(strconv.FormatInt(nextVersion, 10)+".root.json"), maximumMetadataFileBytes, 0)
		if isNotFound(fetchErr) {
			break
		}
		if fetchErr != nil {
			return nil, reject(classifyReleaseError(fetchErr), "trusted root chain is unavailable", fetchErr)
		}
		if _, err := trusted.UpdateRoot(data); err != nil {
			return nil, reject(classifyReleaseError(err), detailInvalidMessage(err), err)
		}
		nextPolicy, err := validateRootPolicy(trusted.Root, in.Local, refTime, &policy)
		if err != nil {
			return nil, reject(classifyReleaseError(err), detailInvalidMessage(err), err)
		}
		policy = nextPolicy
		chain = append(chain, newRootPublication(trusted.Root.Signed.Version, data))
		published := chain[len(chain)-1]
		if err := store.CommitRoot(published.Version, published.Digest, rootBytes(chain)); err != nil {
			return nil, reject(outcomeReleaseInvalid, "publish verified root failed", err)
		}
	}
	probeVersion := trusted.Root.Signed.Version + 1
	if _, probeErr := fetcher.DownloadFile(metadataURL(strconv.FormatInt(probeVersion, 10)+".root.json"), maximumMetadataFileBytes, 0); probeErr == nil {
		return nil, reject(outcomeReleaseInvalid, "root chain exceeds the rotation bound", nil)
	} else if !isNotFound(probeErr) {
		return nil, reject(classifyReleaseError(probeErr), "root rotation probe failed", probeErr)
	}
	timestampBytes, err := fetcher.DownloadFile(metadataURL("timestamp.json"), maximumMetadataFileBytes, 0)
	if err != nil {
		return nil, reject(classifyReleaseError(err), "timestamp metadata is unavailable", err)
	}
	if _, err := trusted.UpdateTimestamp(timestampBytes); err != nil {
		return nil, reject(classifyReleaseError(err), detailInvalidMessage(err), err)
	}
	snapshotMeta := trusted.Timestamp.Signed.Meta[metadata.SNAPSHOT+".json"]
	if snapshotMeta == nil {
		return nil, reject(outcomeReleaseInvalid, "timestamp does not describe snapshot", nil)
	}
	snapshotName := metadata.SNAPSHOT + ".json"
	if trusted.Root.Signed.ConsistentSnapshot {
		snapshotName = strconv.FormatInt(snapshotMeta.Version, 10) + "." + snapshotName
	}
	snapshotBytes, err := fetcher.DownloadFile(metadataURL(snapshotName), metadataLength(snapshotMeta.Length), 0)
	if err != nil {
		return nil, reject(classifyReleaseError(err), "snapshot metadata is unavailable", err)
	}
	if _, err := trusted.UpdateSnapshot(snapshotBytes, false); err != nil {
		return nil, reject(classifyReleaseError(err), detailInvalidMessage(err), err)
	}
	targetsMeta := trusted.Snapshot.Signed.Meta[metadata.TARGETS+".json"]
	if targetsMeta == nil {
		return nil, reject(outcomeReleaseInvalid, "snapshot does not describe targets", nil)
	}
	targetsName := metadata.TARGETS + ".json"
	if trusted.Root.Signed.ConsistentSnapshot {
		targetsName = strconv.FormatInt(targetsMeta.Version, 10) + "." + targetsName
	}
	targetsBytes, err := fetcher.DownloadFile(metadataURL(targetsName), metadataLength(targetsMeta.Length), 0)
	if err != nil {
		return nil, reject(classifyReleaseError(err), "targets metadata is unavailable", err)
	}
	if _, err := trusted.UpdateTargets(targetsBytes); err != nil {
		return nil, reject(classifyReleaseError(err), detailInvalidMessage(err), err)
	}
	if err := validateTrustedShape(*trusted); err != nil {
		return nil, reject(outcomeReleaseInvalid, err.Error(), nil)
	}
	if !fetcher.allFilesUsed() {
		return nil, reject(outcomeReleaseInvalid, "offline import contains unreferenced metadata", nil)
	}
	return &verifiedSet{
		set: *trusted, envelope: fetcher.envelopeUsed(), rootBytes: append([]byte(nil), in.RootBytes...),
		rootChain: chain, timestampBytes: timestampBytes, snapshotBytes: snapshotBytes, targetsBytes: targetsBytes,
	}, Decision{}
}

func rootBytes(chain []rootPublication) [][]byte {
	result := make([][]byte, 0, len(chain))
	for _, root := range chain {
		result = append(result, root.Bytes)
	}
	return result
}

func metadataURL(name string) string { return metadataBaseURL + name }

func metadataLength(declared int64) int64 {
	if declared > 0 {
		return declared
	}
	return maximumMetadataFileBytes
}

func isNotFound(err error) bool {
	var download *metadata.ErrDownloadHTTP
	return errors.As(err, &download) && download.StatusCode == http.StatusNotFound
}

func newRootPublication(version int64, data []byte) rootPublication {
	digest := sha256.Sum256(data)
	return rootPublication{Version: version, Digest: append([]byte(nil), digest[:]...), Bytes: append([]byte(nil), data...)}
}

// metadataRootFromBytes parses a root.json into the go-tuf Root type
// without loading it into a trusted set.
func metadataRootFromBytes(data []byte) (*metadata.Metadata[metadata.RootType], error) {
	root := &metadata.Metadata[metadata.RootType]{}
	return root.FromBytes(data)
}

// validateTrustedShape enforces the Stage 7 profile on the candidate's
// trusted set after Refresh succeeds.
func validateTrustedShape(set trustedmetadata.TrustedMetadata) error {
	if set.Root == nil || set.Timestamp == nil || set.Snapshot == nil {
		return errors.New("trusted set is incomplete")
	}
	if len(set.Root.Signed.Keys) > maximumKeys {
		return errors.New("trusted root key count exceeds the bound")
	}
	if len(set.Root.Signed.Roles) > maximumRoles {
		return errors.New("trusted root role count exceeds the bound")
	}
	if len(set.Root.Signatures) > maximumSignatures {
		return errors.New("root signature count exceeds the bound")
	}
	if len(set.Timestamp.Signatures) > maximumSignatures {
		return errors.New("timestamp signature count exceeds the bound")
	}
	if len(set.Snapshot.Signatures) > maximumSignatures {
		return errors.New("snapshot signature count exceeds the bound")
	}
	if len(set.Snapshot.Signed.Meta) > maximumRoles {
		return errors.New("snapshot meta count exceeds the bound")
	}
	if len(set.Targets) != 1 {
		return errors.New("only the top-level targets role is allowed")
	}
	top, ok := set.Targets[metadata.TARGETS]
	if !ok || top == nil {
		return errors.New("only the top-level targets role is allowed")
	}
	if top.Signed.Delegations != nil {
		return errors.New("delegated targets are disabled")
	}
	if len(top.Signatures) > maximumSignatures {
		return errors.New("targets signature count exceeds the bound")
	}
	if len(top.Signed.Targets) > maximumTargets {
		return errors.New("targets description count exceeds the bound")
	}
	return nil
}
