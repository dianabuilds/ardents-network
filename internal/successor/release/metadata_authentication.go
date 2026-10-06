package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const targetRole = "targets"

const metadataBaseURL = "https://release.invalid/metadata/"

type verifiedSet struct {
	set            trustedmetadata.TrustedMetadata
	envelope       envelopeState
	rootBytes      []byte
	rootChain      []rootPublication
	timestampBytes []byte
	snapshotBytes  []byte
	targetsBytes   []byte
}

type rootPublication struct {
	Version int64
	Digest  []byte
	Bytes   []byte
}

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

func metadataRootFromBytes(data []byte) (*metadata.Metadata[metadata.RootType], error) {
	if _, err := parseMetadataEnvelope(data); err != nil {
		return nil, err
	}
	root := &metadata.Metadata[metadata.RootType]{}
	return root.FromBytes(data)
}

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

type fetcherAdapter struct {
	mu       sync.Mutex
	base     *url.URL
	files    map[string][]byte
	used     map[string]struct{}
	envelope envelopeState
}

type envelopeState struct {
	fetches int
	bytes   int64
}

func newMapFetcher(base string, files map[string][]byte) (*fetcherAdapter, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("release: parse base URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("release: base URL is outside the offline envelope")
	}
	parsed.Path = strings.TrimSuffix(path.Clean(parsed.Path), "/") + "/"
	return &fetcherAdapter{base: parsed, files: copyFiles(files), used: make(map[string]struct{}), envelope: envelopeState{}}, nil
}

func copyFiles(files map[string][]byte) map[string][]byte {
	dup := make(map[string][]byte, len(files))
	for key, value := range files {
		dup[key] = value
	}
	return dup
}

func (f *fetcherAdapter) DownloadFile(raw string, maxLength int64, _ time.Duration) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envelope.fetches++
	if f.envelope.fetches > maximumFetches {
		return nil, fmt.Errorf("release: fetch count exceeds %d", maximumFetches)
	}
	if maxLength < 0 || maxLength > maximumMetadataFileBytes {
		return nil, fmt.Errorf("release: requested length %d exceeds the bound", maxLength)
	}
	request, err := url.Parse(raw)
	if err != nil || request.Scheme != f.base.Scheme || request.Host != f.base.Host ||
		request.User != nil || request.RawQuery != "" || request.Fragment != "" {
		return nil, errors.New("release: URL is outside the offline envelope")
	}
	decoded, err := url.PathUnescape(request.EscapedPath())
	if err != nil || !strings.HasPrefix(decoded, f.base.Path) {
		return nil, errors.New("release: URL path is outside the offline envelope")
	}
	cleaned := path.Clean(decoded)
	if cleaned != decoded || strings.Contains(decoded, `\`) {
		return nil, errors.New("release: URL path is not confined")
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return nil, errors.New("release: URL path escapes the offline envelope")
		}
	}
	data, ok := f.files[raw]
	if !ok {
		// The TUF client uses a 404 to signal "no newer root" or
		// "missing consistent-snapshot variant". Return the typed
		// error the client expects so Refresh can complete.
		return nil, &metadata.ErrDownloadHTTP{StatusCode: 404, URL: raw}
	}
	f.used[raw] = struct{}{}
	if int64(len(data)) > maxLength || int64(len(data)) > maximumMetadataFileBytes {
		return nil, &releaseError{class: outcomeReleaseInvalid, message: "metadata file exceeds the per-file bound"}
	}
	if f.envelope.bytes+int64(len(data)) > maximumMetadataBytes {
		return nil, &releaseError{class: outcomeReleaseInvalid, message: "metadata aggregate exceeds the bound"}
	}
	f.envelope.bytes += int64(len(data))
	return append([]byte(nil), data...), nil
}

func (f *fetcherAdapter) allFilesUsed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.used) == len(f.files)
}

func (f *fetcherAdapter) envelopeUsed() envelopeState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return envelopeState{fetches: f.envelope.fetches, bytes: f.envelope.bytes}
}

type releaseError struct {
	class   Outcome
	message string
}

func (e *releaseError) Error() string {
	return e.message
}

func classifyReleaseError(err error) Outcome {
	if err == nil {
		return outcomeReleaseAccepted
	}
	var typed *releaseError
	if errors.As(err, &typed) {
		return typed.class
	}
	var expired *metadata.ErrExpiredMetadata
	if errors.As(err, &expired) {
		return outcomeReleaseExpired
	}
	if strings.Contains(err.Error(), "exceeds the bound") {
		return outcomeReleaseInvalid
	}
	if strings.Contains(err.Error(), "URL") || strings.Contains(err.Error(), "offline envelope") {
		return outcomeReleaseInvalid
	}
	if strings.Contains(err.Error(), "exceeds") {
		return outcomeReleaseInvalid
	}
	return outcomeReleaseInvalid
}

type metadataEnvelope struct {
	role        string
	keys        map[string]struct{}
	roles       map[string]struct{}
	delegations bool
}

func validateInputsEnvelope(in Inputs) error {
	if int64(len(in.RootBytes)) > maximumMetadataFileBytes {
		return errors.New("trusted root exceeds the per-file bound")
	}
	if in.TargetPath == "" {
		return errors.New("target path is missing")
	}
	if len(in.Artifact) == 0 {
		return errors.New("artifact bytes are missing")
	}
	if int64(len(in.Artifact)) > maximumArtifactBytes {
		return errors.New("artifact bytes exceed the bound")
	}
	if in.Files == nil {
		return errors.New("metadata files are missing")
	}
	aggregate := int64(len(in.RootBytes))
	fetches := 0
	keys := make(map[string]struct{})
	roles := make(map[string]struct{})
	signatures := make(map[string]struct{})
	hasDelegations := false
	for path, data := range in.Files {
		fetches++
		if fetches > maximumFetches {
			return errors.New("file count exceeds the fetch bound")
		}
		if int64(len(data)) > maximumMetadataFileBytes {
			return fmt.Errorf("file %q exceeds the per-file bound", path)
		}
		aggregate += int64(len(data))
		if aggregate > maximumMetadataBytes {
			return errors.New("aggregate metadata exceeds the bound")
		}
		envelope, err := parseMetadataEnvelope(data)
		if err != nil {
			return fmt.Errorf("file %q is not a recognized metadata envelope: %w", path, err)
		}
		for key := range envelope.keys {
			keys[key] = struct{}{}
		}
		for role := range envelope.roles {
			roles[role] = struct{}{}
		}
		signatures[envelope.role] = struct{}{}
		if envelope.delegations {
			hasDelegations = true
		}
	}
	if len(keys) > maximumKeys {
		return errors.New("key count exceeds the bound")
	}
	if len(roles) > maximumRoles {
		return errors.New("role count exceeds the bound")
	}
	if len(signatures) > maximumSignatures {
		return errors.New("signature role count exceeds the bound")
	}
	if hasDelegations {
		return errors.New("delegated targets are disabled")
	}
	return nil
}

func parseMetadataEnvelope(data []byte) (metadataEnvelope, error) {
	if err := validateJSONObjects(data, false); err != nil {
		return metadataEnvelope{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return metadataEnvelope{}, err
	}
	if len(fields) != 2 || fields["signed"] == nil || fields["signatures"] == nil {
		return metadataEnvelope{}, errors.New("metadata envelope fields are not canonical")
	}
	var envelope struct {
		Signed     json.RawMessage   `json:"signed"`
		Signatures []json.RawMessage `json:"signatures"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return metadataEnvelope{}, err
	}
	if len(envelope.Signatures) > maximumSignatures {
		return metadataEnvelope{}, errors.New("signature count exceeds the bound")
	}
	var signed struct {
		Type        string                     `json:"_type"`
		Keys        map[string]json.RawMessage `json:"keys"`
		Roles       map[string]json.RawMessage `json:"roles"`
		Meta        map[string]json.RawMessage `json:"meta"`
		Targets     map[string]json.RawMessage `json:"targets"`
		Delegations json.RawMessage            `json:"delegations"`
	}
	if err := json.Unmarshal(envelope.Signed, &signed); err != nil {
		return metadataEnvelope{}, err
	}
	keys := make(map[string]struct{}, len(signed.Keys))
	for key := range signed.Keys {
		keys[key] = struct{}{}
	}
	roles := make(map[string]struct{}, len(signed.Roles))
	for role := range signed.Roles {
		roles[role] = struct{}{}
	}
	if signed.Type == targetRole && len(signed.Delegations) > 0 && string(signed.Delegations) != "null" {
		return metadataEnvelope{role: signed.Type, keys: keys, roles: roles, delegations: true}, nil
	}
	return metadataEnvelope{role: signed.Type, keys: keys, roles: roles}, nil
}

// A signed object must have one interpretation. encoding/json otherwise
// accepts duplicate object keys, including escaped equivalents. Custom
// identities additionally use only their exact lowercase schema field names.
func validateJSONObjects(data []byte, lowercaseKeys bool) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value func() error
	value = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, nested := token.(json.Delim)
		if !nested {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				token, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := token.(string)
				if !ok {
					return errors.New("invalid JSON object key")
				}
				if _, duplicate := seen[key]; duplicate {
					return errors.New("duplicate JSON object key")
				}
				if lowercaseKeys && key != strings.ToLower(key) {
					return errors.New("noncanonical custom identity key")
				}
				seen[key] = struct{}{}
				if err := value(); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := value(); err != nil {
					return err
				}
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON value")
	}
	return nil
}

const (
	rootSchemaField  = "ardents_schema_version"
	rootProfileField = "ardents_profile"
	rootEnvField     = "ardents_environment"
	rootNetworkField = "ardents_network"
)

type rootPolicy struct {
	environment string
	network     string
}

func validateRootPolicy(root *metadata.Metadata[metadata.RootType], local LocalEnvironment, refTime time.Time, previous *rootPolicy) (rootPolicy, error) {
	if root == nil {
		return rootPolicy{}, errors.New("trusted root is missing")
	}
	if root.Signed.IsExpired(refTime) {
		if previous != nil {
			return rootPolicy{}, errors.New("trusted root rotation contains an expired candidate")
		}
		return rootPolicy{}, &metadata.ErrExpiredMetadata{Msg: "trusted root is expired"}
	}
	fields := root.Signed.UnrecognizedFields
	if len(fields) != 4 || integerField(fields[rootSchemaField]) != targetSchemaVersion || fields[rootProfileField] != targetProfile {
		return rootPolicy{}, errors.New("trusted root schema or profile is invalid")
	}
	environment, environmentOK := fields[rootEnvField].(string)
	network, networkOK := fields[rootNetworkField].(string)
	if !environmentOK || !networkOK || environment == "" || network == "" {
		return rootPolicy{}, errors.New("trusted root environment binding is invalid")
	}
	if environment != local.Environment || network != local.Network {
		return rootPolicy{}, errors.New("trusted root does not match the local environment")
	}
	if previous != nil && (environment != previous.environment || network != previous.network) {
		return rootPolicy{}, errors.New("trusted root rotation changes environment or network")
	}
	for _, name := range metadata.TOP_LEVEL_ROLE_NAMES {
		role := root.Signed.Roles[name]
		if role == nil || len(role.KeyIDs) != totalTopLevelKeys || role.Threshold != ordinaryThreshold {
			return rootPolicy{}, fmt.Errorf("trusted root role %s has an invalid threshold", name)
		}
		seen := make(map[string]struct{}, totalTopLevelKeys)
		for _, keyID := range role.KeyIDs {
			key := root.Signed.Keys[keyID]
			if key == nil {
				return rootPolicy{}, fmt.Errorf("trusted root role %s refers to an unknown key", name)
			}
			if _, duplicate := seen[keyID]; duplicate {
				return rootPolicy{}, errors.New("trusted root role repeats a key")
			}
			canonicalID, err := key.ID()
			if err != nil || canonicalID != keyID {
				return rootPolicy{}, errors.New("trusted root key identity differs from its public material")
			}
			seen[keyID] = struct{}{}
		}
	}
	return rootPolicy{environment: environment, network: network}, nil
}

func integerField(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case float64:
		if number != math.Trunc(number) {
			return 0
		}
		return int(number)
	default:
		return 0
	}
}

type rootRotationResult struct {
	// advanced reports whether the candidate advanced beyond the durable
	// floor. Equal versions are still accepted when digest matches.
	advanced bool
	// root is the final trusted root in the chain.
	root rootPublication
	// conflict reports whether the candidate contradicted the durable
	// floor.
	conflict bool
}

func checkRootRotation(chain []rootPublication, durable FloorSet) (rootRotationResult, error) {
	if len(chain) == 0 {
		return rootRotationResult{}, errors.New("root chain is empty")
	}
	if len(chain) > int(maximumRootRotations)+1 {
		return rootRotationResult{}, fmt.Errorf("root chain exceeds the rotation bound of %d", maximumRootRotations)
	}
	first := chain[0]
	if durable.RootVersion == 0 && first.Version != 1 {
		return rootRotationResult{}, fmt.Errorf("root chain does not start at version 1, got %d", first.Version)
	}
	if durable.RootVersion != 0 {
		if first.Version != durable.RootVersion || !bytes.Equal(first.Digest, durable.RootDigest) {
			return rootRotationResult{conflict: first.Version == durable.RootVersion}, errors.New("initial root does not match the durable floor")
		}
	}
	previousVersion := first.Version
	previousDigest := first.Digest
	for index := 1; index < len(chain); index++ {
		current := chain[index]
		if current.Version != previousVersion+1 {
			return rootRotationResult{}, fmt.Errorf("root chain has a gap between version %d and %d", previousVersion, current.Version)
		}
		if bytes.Equal(current.Digest, previousDigest) {
			return rootRotationResult{}, errors.New("root chain reuses a previous digest")
		}
		previousVersion = current.Version
		previousDigest = current.Digest
	}
	final := chain[len(chain)-1]
	if durable.RootVersion != 0 {
		if final.Version < durable.RootVersion {
			return rootRotationResult{}, errors.New("final root is older than the durable floor")
		}
		if final.Version == durable.RootVersion && !bytes.Equal(final.Digest, durable.RootDigest) {
			return rootRotationResult{conflict: true}, errors.New("final root disagrees with the durable floor at the same version")
		}
	}
	return rootRotationResult{advanced: final.Version > durable.RootVersion, root: final}, nil
}

func successorFloors(set *verifiedSet, rotation rootRotationResult) (FloorSet, error) {
	if set == nil {
		return FloorSet{}, errors.New("trusted set is missing")
	}
	rootDigest := sha256.Sum256(set.rootBytes)
	if len(rotation.root.Digest) == 32 {
		rootDigest = [32]byte(rotation.root.Digest)
	}
	if len(set.timestampBytes) == 0 || len(set.snapshotBytes) == 0 || len(set.targetsBytes) == 0 {
		return FloorSet{}, errors.New("verified raw metadata is incomplete")
	}
	timestampDigest := sha256.Sum256(set.timestampBytes)
	snapshotDigest := sha256.Sum256(set.snapshotBytes)
	targetsDigest := sha256.Sum256(set.targetsBytes)
	return FloorSet{
		RootVersion:      set.set.Root.Signed.Version,
		RootDigest:       rootDigest[:],
		TimestampVersion: set.set.Timestamp.Signed.Version,
		TimestampDigest:  timestampDigest[:],
		SnapshotVersion:  set.set.Snapshot.Signed.Version,
		SnapshotDigest:   snapshotDigest[:],
		TargetsVersion:   set.set.Targets[targetRole].Signed.Version,
		TargetsDigest:    targetsDigest[:],
	}, nil
}
