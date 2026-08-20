//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
)

const (
	profileMetadataFileBytes = int64(1 << 20)
	profileMetadataBytes     = int64(8 << 20)
	profileFetches           = 32
	profileRootRotations     = int64(16)
	profileRoles             = 32
	profileKeys              = 64
	profileSignatures        = 64
	profileTargets           = 1024
)

var (
	errProfileAggregate = errors.New("release metadata aggregate exceeds profile")
	errProfileCount     = errors.New("release metadata count exceeds profile")
	errProfileURL       = errors.New("release metadata URL is outside profile")
)

// boundedFetcher is the experiment form of the Stage 7 byte Adapter. It owns
// no network client and makes no trust decision: the candidate still receives
// the exact bytes and the smaller of its requested limit and the Ardents cap.
type boundedFetcher struct {
	mu       sync.Mutex
	base     *url.URL
	files    map[string][]byte
	fetches  int
	total    int64
	maxTotal int64
}

var _ fetcher.Fetcher = (*boundedFetcher)(nil)

func newBoundedFetcher(base string, files map[string][]byte) (*boundedFetcher, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errProfileURL
	}
	parsed.Path = strings.TrimSuffix(path.Clean(parsed.Path), "/") + "/"
	return &boundedFetcher{base: parsed, files: files, maxTotal: profileMetadataBytes}, nil
}

func (f *boundedFetcher) DownloadFile(raw string, maxLength int64, _ time.Duration) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.fetches++
	if f.fetches > profileFetches {
		return nil, errProfileCount
	}
	if maxLength < 0 || maxLength > profileMetadataFileBytes {
		return nil, errProfileCount
	}
	request, err := url.Parse(raw)
	if err != nil || request.Scheme != f.base.Scheme || request.Host != f.base.Host || request.User != nil || request.RawQuery != "" || request.Fragment != "" {
		return nil, errProfileURL
	}
	decodedPath, err := url.PathUnescape(request.EscapedPath())
	if err != nil || !strings.HasPrefix(decodedPath, f.base.Path) || path.Clean(decodedPath) != decodedPath || strings.Contains(decodedPath, `\`) {
		return nil, errProfileURL
	}
	for _, segment := range strings.Split(decodedPath, "/") {
		if segment == "." || segment == ".." {
			return nil, errProfileURL
		}
	}
	data, ok := f.files[raw]
	if !ok {
		return nil, &metadata.ErrDownloadHTTP{StatusCode: 404, URL: raw}
	}
	if int64(len(data)) > maxLength || int64(len(data)) > profileMetadataFileBytes {
		return nil, &metadata.ErrDownloadLengthMismatch{Msg: "release metadata exceeds per-file profile"}
	}
	if err = validateMetadataEnvelope(data); err != nil {
		return nil, err
	}
	if f.total+int64(len(data)) > f.maxTotal {
		return nil, errProfileAggregate
	}
	f.total += int64(len(data))
	return append([]byte(nil), data...), nil
}

func newProfileConfig(root []byte, source fetcher.Fetcher) (*config.UpdaterConfig, error) {
	if int64(len(root)) > profileMetadataFileBytes {
		return nil, &metadata.ErrDownloadLengthMismatch{Msg: "trusted root exceeds per-file profile"}
	}
	if err := validateMetadataEnvelope(root); err != nil {
		return nil, err
	}
	if bounded, ok := source.(*boundedFetcher); ok {
		bounded.mu.Lock()
		if bounded.total+int64(len(root)) > bounded.maxTotal {
			bounded.mu.Unlock()
			return nil, errProfileAggregate
		}
		bounded.total += int64(len(root))
		bounded.mu.Unlock()
	}
	cfg, err := config.New("https://release.invalid/metadata", root)
	if err != nil {
		return nil, err
	}
	cfg.MaxRootRotations = profileRootRotations
	// Stage 7 has one top-level release role. Delegated targets are not needed,
	// so disabling their traversal removes a depth-policy seam from the profile.
	cfg.MaxDelegations = 0
	cfg.RootMaxLength = profileMetadataFileBytes
	cfg.TimestampMaxLength = profileMetadataFileBytes
	cfg.SnapshotMaxLength = profileMetadataFileBytes
	cfg.TargetsMaxLength = profileMetadataFileBytes
	cfg.Fetcher = source
	cfg.DisableLocalCache = true
	cfg.LocalMetadataDir = ""
	cfg.LocalTargetsDir = ""
	cfg.PrefixTargetsWithHash = true
	cfg.UnsafeLocalMode = false
	return cfg, nil
}

// validateMetadataEnvelope is a reject-only resource preflight. It does not
// verify or normalize TUF data and cannot turn rejected candidate input into
// accepted input. The candidate remains the sole TUF workflow implementation.
func validateMetadataEnvelope(data []byte) error {
	var envelope struct {
		Signed     json.RawMessage   `json:"signed"`
		Signatures []json.RawMessage `json:"signatures"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Signatures) > profileSignatures {
		return errProfileCount
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
		return err
	}
	if len(signed.Keys) > profileKeys || len(signed.Roles) > profileRoles || len(signed.Meta) > profileRoles || len(signed.Targets) > profileTargets {
		return errProfileCount
	}
	if signed.Type == metadata.TARGETS && len(signed.Delegations) > 0 && string(signed.Delegations) != "null" {
		return errors.New("delegated targets are disabled")
	}
	return nil
}

func validateProfileShape(set trustedmetadata.TrustedMetadata) error {
	if set.Root == nil || set.Timestamp == nil || set.Snapshot == nil {
		return errors.New("incomplete trusted metadata set")
	}
	if len(set.Root.Signed.Keys) > profileKeys || len(set.Root.Signatures) > profileSignatures || len(set.Timestamp.Signatures) > profileSignatures || len(set.Snapshot.Signatures) > profileSignatures {
		return errProfileCount
	}
	if len(set.Root.Signed.Roles) != len(metadata.TOP_LEVEL_ROLE_NAMES) {
		return errors.New("unexpected top-level role set")
	}
	if len(set.Snapshot.Signed.Meta) > profileRoles || len(set.Targets) > profileRoles {
		return errProfileCount
	}
	top, ok := set.Targets[metadata.TARGETS]
	if !ok || top == nil || len(set.Targets) != 1 {
		return errors.New("delegated targets are disabled")
	}
	if top.Signed.Delegations != nil {
		return errors.New("delegated targets are disabled")
	}
	if len(top.Signatures) > profileSignatures || len(top.Signed.Targets) > profileTargets {
		return errProfileCount
	}
	return nil
}

func verifyArtifact(info *metadata.TargetFiles, artifact []byte) error {
	if info == nil {
		return errors.New("missing target information")
	}
	return info.VerifyLengthHashes(artifact)
}

func verifyRepositoryDecision(root []byte, files map[string][]byte, targetPath string, artifact []byte) error {
	source, err := newBoundedFetcher("https://release.invalid/metadata/", files)
	if err != nil {
		return err
	}
	cfg, err := newProfileConfig(root, source)
	if err != nil {
		return err
	}
	client, err := updater.New(cfg)
	if err != nil {
		return err
	}
	if err = client.Refresh(); err != nil {
		return err
	}
	if err = validateProfileShape(client.GetTrustedMetadataSet()); err != nil {
		return err
	}
	target, err := client.GetTargetInfo(targetPath)
	if err != nil {
		return err
	}
	return verifyArtifact(target, artifact)
}

func maximumProfileEnvelope() (trustedmetadata.TrustedMetadata, *metadata.TargetFiles, []byte) {
	expires := time.Now().UTC().Add(24 * time.Hour)
	root := metadata.Root(expires)
	root.Signatures = syntheticSignatures(profileSignatures)
	for index := 0; index < profileKeys; index++ {
		id := fmt.Sprintf("key-%02d", index)
		root.Signed.Keys[id] = &metadata.Key{Type: "ed25519", Scheme: "ed25519", Value: metadata.KeyVal{PublicKey: fmt.Sprintf("public-%02d", index)}}
	}
	timestamp := metadata.Timestamp(expires)
	timestamp.Signatures = syntheticSignatures(profileSignatures)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signatures = syntheticSignatures(profileSignatures)
	for index := 1; index < profileRoles; index++ {
		snapshot.Signed.Meta[fmt.Sprintf("unused-%02d.json", index)] = &metadata.MetaFiles{Version: 1}
	}
	targets := metadata.Targets(expires)
	targets.Signatures = syntheticSignatures(profileSignatures)
	artifact := make([]byte, profileMetadataFileBytes)
	digest := sha256.Sum256(artifact)
	selected := &metadata.TargetFiles{Length: int64(len(artifact)), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: "ardents/windows-amd64/application"}
	for index := 0; index < profileTargets; index++ {
		info := &metadata.TargetFiles{Length: 1, Hashes: metadata.Hashes{"sha256": make([]byte, sha256.Size)}, Path: fmt.Sprintf("target-%04d", index)}
		targets.Signed.Targets[info.Path] = info
	}
	targets.Signed.Targets[selected.Path] = selected
	delete(targets.Signed.Targets, "target-0000")
	return trustedmetadata.TrustedMetadata{
		Root: root, Timestamp: timestamp, Snapshot: snapshot,
		Targets: map[string]*metadata.Metadata[metadata.TargetsType]{metadata.TARGETS: targets},
		RefTime: time.Now().UTC(),
	}, selected, artifact
}

func syntheticSignatures(count int) []metadata.Signature {
	result := make([]metadata.Signature, count)
	for index := range result {
		result[index] = metadata.Signature{KeyID: fmt.Sprintf("key-%02d", index), Signature: metadata.HexBytes{byte(index)}}
	}
	return result
}
