package releasedecision

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
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
	updater   *updater.Updater
	set       trustedmetadata.TrustedMetadata
	envelope  envelopeState
	rootBytes []byte
	rootChain []rootPublication
}

// rootPublication is one verified root's bytes plus its version and
// digest for the durable floor.
type rootPublication struct {
	Version int64
	Digest  []byte
	Bytes   []byte
}

// buildVerifiedSet constructs a go-tuf updater, runs Refresh, and
// returns the verified trusted set plus the bounded envelope.
func buildVerifiedSet(ctx context.Context, in Inputs, refTime time.Time) (*verifiedSet, Decision) {
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
	cfg, err := newReleaseConfig(in.RootBytes, fetcher)
	if err != nil {
		return nil, reject(outcomeReleaseInvalid, err.Error(), err)
	}
	client, err := updater.New(cfg)
	if err != nil {
		return nil, reject(outcomeReleaseInvalid, "updater construction failed", err)
	}
	if err := client.Refresh(); err != nil {
		outcome := classifyReleaseError(err)
		notice := "trusted root chain failed to verify"
		if outcome == outcomeReleaseInvalid {
			notice = detailInvalidMessage(err)
		}
		return nil, reject(outcome, notice, err)
	}
	rawSet := client.GetTrustedMetadataSet()
	if err := validateTrustedShape(rawSet); err != nil {
		return nil, reject(outcomeReleaseInvalid, err.Error(), nil)
	}
	rawSet.RefTime = refTime
	chain, err := captureRootChain(in, rawSet)
	if err != nil {
		return nil, reject(outcomeReleaseInvalid, err.Error(), err)
	}
	return &verifiedSet{updater: client, set: rawSet, envelope: fetcher.envelopeUsed(), rootBytes: append([]byte(nil), in.RootBytes...), rootChain: chain}, Decision{}
}

// detailInvalidMessage shortens common go-tuf error messages into
// stable, non-sensitive Notice strings.
func detailInvalidMessage(err error) string {
	if err == nil {
		return "release metadata is invalid"
	}
	text := err.Error()
	switch {
	case strings.Contains(text, "expired"):
		return "metadata is expired"
	case strings.Contains(text, "version"):
		return "metadata version is not authorized"
	case strings.Contains(text, "threshold"):
		return "metadata signature threshold is not met"
	case strings.Contains(text, "length"):
		return "metadata length is not authorized"
	case strings.Contains(text, "hash"):
		return "metadata hash is not authorized"
	case strings.Contains(text, "delegat"):
		return "delegated targets are disabled"
	case strings.Contains(text, "root"):
		return "trusted root is not authorized"
	}
	return "release metadata is invalid"
}

// metadataRootFromBytes parses a root.json into the go-tuf Root type
// without loading it into a trusted set.
func metadataRootFromBytes(data []byte) (*metadata.Metadata[metadata.RootType], error) {
	root := &metadata.Metadata[metadata.RootType]{}
	return root.FromBytes(data)
}

// newReleaseConfig builds the go-tuf config with the R-049 frozen
// profile.
func newReleaseConfig(root []byte, fetcher *fetcherAdapter) (*config.UpdaterConfig, error) {
	cfg, err := config.New(metadataBaseURL, root)
	if err != nil {
		return nil, err
	}
	cfg.MaxRootRotations = maximumRootRotations
	cfg.MaxDelegations = maximumDelegations
	cfg.RootMaxLength = maximumMetadataFileBytes
	cfg.TimestampMaxLength = maximumMetadataFileBytes
	cfg.SnapshotMaxLength = maximumMetadataFileBytes
	cfg.TargetsMaxLength = maximumMetadataFileBytes
	cfg.Fetcher = fetcher
	cfg.DisableLocalCache = true
	cfg.LocalMetadataDir = ""
	cfg.LocalTargetsDir = ""
	cfg.PrefixTargetsWithHash = true
	cfg.UnsafeLocalMode = false
	return cfg, nil
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

// captureRootChain records the trusted root bytes for atomic
// publication.
func captureRootChain(in Inputs, set trustedmetadata.TrustedMetadata) ([]rootPublication, error) {
	chain := make([]rootPublication, 0, 2)
	rootDigest := sha256.Sum256(in.RootBytes)
	chain = append(chain, rootPublication{Version: set.Root.Signed.Version, Digest: rootDigest[:], Bytes: append([]byte(nil), in.RootBytes...)})
	return chain, nil
}

// reject produces a failed Decision with the supplied outcome, a
// stable Notice, and the underlying error wrapped in errInvalidInputs.
func reject(outcome Outcome, notice string, cause error) Decision {
	combined := notice
	if cause != nil {
		combined = fmt.Sprintf("%s: %v", notice, cause)
	}
	return Decision{Outcome: outcome, Notice: combined}
}
