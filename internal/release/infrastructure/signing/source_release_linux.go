package signing

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func runSourceReleaseSigning(args []string, out io.Writer) error {
	return runSourceReleasePurpose(args, out, sourceReleaseInitial)
}

func runSourceReleasePurpose(args []string, out io.Writer, purpose sourceReleasePurpose) error {
	wanted, operation := 4, "sign-initial-release"
	if purpose == sourceReleaseForward {
		wanted, operation = 5, "sign-forward-release"
	} else if purpose == sourceReleaseTimestamp {
		wanted, operation = 5, "refresh-release-timestamp"
	} else if purpose != sourceReleaseInitial {
		return artifactio.ErrInput
	}
	if len(args) != wanted {
		if purpose == sourceReleaseForward {
			return errors.New("usage: ardents infrastructure sign-release source sign-forward-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION PREDECESSOR_DISTRIBUTION")
		}
		if purpose == sourceReleaseTimestamp {
			return errors.New("usage: ardents infrastructure sign-release source refresh-release-timestamp PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION CURRENT_DISTRIBUTION")
		}
		return errors.New("usage: ardents infrastructure sign-release source sign-initial-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	root, err := signSourceRelease(ctx, args, purpose, nil)
	if err != nil {
		return artifactio.Refuse(err, "source control failed")
	}
	digest := sha256.Sum256(root)
	raw, err := json.Marshal(struct {
		Operation           string   `json:"operation"`
		Root                string   `json:"root_sha256"`
		Targets             []string `json:"targets"`
		Installed           bool     `json:"installed_qualification"`
		IndependentCustody  bool     `json:"independent_custody"`
		IndependentBuilders bool     `json:"independent_builders"`
	}{"source." + operation, hex.EncodeToString(digest[:]), []string{infrastructure.ProgramTarget, infrastructure.GenerationTarget}, false, false, false})
	if err != nil {
		return err
	}
	return artifactio.WriteReceipt(ctx, out, append(raw, '\n'))
}

// Original private keys never enter Release/Installation. This one root
// operation admits only the two purpose-bound Source targets, obtains actual Release
// decisions under one diagnostic verifier, then writes public metadata after
// all original input and physical cleanup checks. Partial outputs stay retained.
func signSourceRelease(ctx context.Context, args []string, purpose sourceReleasePurpose, observe func(string, *os.File)) (result []byte, returned error) {
	wantedArguments := 4
	if purpose == sourceReleaseForward || purpose == sourceReleaseTimestamp {
		wantedArguments = 5
	} else if purpose != sourceReleaseInitial {
		return nil, artifactio.ErrInput
	}
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) != wantedArguments {
		return nil, artifactio.ErrInput
	}
	for _, path := range args {
		if !artifactio.AbsolutePath(path) {
			return nil, artifactio.ErrInput
		}
	}
	for i, path := range args[1:] {
		for _, previous := range args[1 : i+1] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return nil, artifactio.ErrInput
			}
		}
	}
	inputs := artifactio.NewInputs(ctx)
	defer func() {
		returned = errors.Join(returned, inputs.Close(), ctx.Err())
		if returned != nil {
			result = nil
		}
	}()
	planRaw, err := inputs.Read(args[0], 1<<20)
	if err != nil {
		return nil, err
	}
	plan, err := decodeSourceReleasePurpose(planRaw, time.Now().UTC(), purpose)
	if err != nil {
		return nil, err
	}
	var artifacts [3][]byte
	for i, path := range []string{plan.Program, plan.Descriptor, plan.Template} {
		for _, root := range args[1:] {
			if path == root || strings.HasPrefix(path, root+"/") {
				return nil, artifactio.ErrInput
			}
		}
		maximum := int64(64 << 20)
		if i == 1 {
			maximum = 16 << 10
		}
		artifacts[i], err = inputs.Read(path, maximum)
		if err != nil {
			return nil, err
		}
	}
	if err := validateSourceReleaseResources(plan, artifacts[0], artifacts[1], artifacts[2]); err != nil {
		return nil, err
	}
	var predecessor *sourceReleasePredecessor
	if purpose != sourceReleaseInitial {
		predecessor, err = readSourceReleasePredecessor(ctx, inputs, args[4], plan, artifacts[1])
		if err != nil {
			return nil, err
		}
		if purpose == sourceReleaseTimestamp {
			if err := predecessor.checkTimestampResources(plan, artifacts); err != nil {
				return nil, err
			}
		}
	}
	keyDirectory, err := inputs.Directory(args[1])
	if err != nil {
		return nil, err
	}
	if keyDirectory.Info().Mode() != os.ModeDir|0700 {
		return nil, artifactio.ErrInput
	}
	names, err := keyDirectory.Descriptor().Readdirnames(6)
	if err != nil && err != io.EOF || len(names) != 5 {
		return nil, artifactio.ErrInput
	}
	wanted := map[string]bool{}
	for i := 1; i <= 5; i++ {
		wanted[fmt.Sprintf("release-key-%d.pem", i)] = true
	}
	for _, name := range names {
		if !wanted[name] {
			return nil, artifactio.ErrInput
		}
	}
	// Retain both future-output parent namespaces before private-key use.
	var outputParents [2]*artifactio.File
	for i, path := range args[2:4] {
		parent, err := inputs.Directory(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		if parent.Info().Mode() != os.ModeDir|0700 {
			return nil, artifactio.ErrInput
		}
		var fs syscall.Statfs_t
		if err := syscall.Fstatfs(int(parent.Descriptor().Fd()), &fs); err != nil || fs.Type != 0xef53 {
			return nil, errors.Join(artifactio.ErrInput, err)
		}
		probe, err := artifactio.OpenChild(parent, filepath.Base(path), false)
		if probe != nil {
			return nil, errors.Join(artifactio.ErrInput, probe.Close())
		}
		if !errors.Is(err, syscall.ENOENT) {
			return nil, errors.Join(artifactio.ErrInput, err)
		}
		outputParents[i] = parent
	}
	check := func() error {
		now := time.Now().UTC()
		if plan.Reference.Before(now.Add(-5*time.Minute)) || plan.Reference.After(now.Add(time.Minute)) || !now.Before(plan.Expires) {
			return artifactio.ErrInput
		}
		if _, err := keyDirectory.Descriptor().Seek(0, io.SeekStart); err != nil {
			return err
		}
		names, err := keyDirectory.Descriptor().Readdirnames(6)
		if err != nil && err != io.EOF || len(names) != 5 {
			return artifactio.ErrInput
		}
		for _, name := range names {
			if !wanted[name] {
				return artifactio.ErrInput
			}
		}
		if predecessor != nil {
			if err := predecessor.check(); err != nil {
				return err
			}
		}
		return inputs.CheckProtected()
	}
	if err := check(); err != nil {
		return nil, err
	}
	root := metadata.Root(plan.Expires)
	root.Signed.UnrecognizedFields = map[string]any{"ardents_schema_version": 1, "ardents_profile": "ardents-h3-release-v1", "ardents_environment": "alpha", "ardents_network": plan.Network}
	var keys []ed25519.PrivateKey
	defer func() {
		for _, key := range keys {
			clear(key)
		}
	}()
	var signers []signature.Signer
	var keyIDs []string
	for i := 1; i <= 5; i++ {
		raw, err := inputs.Read(filepath.Join(args[1], fmt.Sprintf("release-key-%d.pem", i)), 4096)
		if err != nil {
			return nil, err
		}
		info := inputs.Last().Info()
		if info.Mode() != 0600 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			return nil, artifactio.ErrInput
		}
		key, err := artifactio.ParseEd25519Key(raw)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		signer, err := signature.LoadSigner(key, crypto.Hash(0))
		if err != nil {
			return nil, err
		}
		signers = append(signers, signer)
		public, err := metadata.KeyFromPublicKey(key.Public())
		if err != nil {
			return nil, err
		}
		id, err := public.ID()
		if err != nil {
			return nil, err
		}
		if _, exists := root.Signed.Keys[id]; exists {
			return nil, artifactio.ErrInput
		}
		root.Signed.Keys[id] = public
		keyIDs = append(keyIDs, id)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), keyIDs...), Threshold: 3}
	}
	var diagnostic *release.Verifier
	defer func() {
		if diagnostic != nil {
			returned = errors.Join(returned, diagnostic.Close())
		}
	}()
	if predecessor != nil {
		if err := predecessor.matchKeys(keyIDs); err != nil {
			return nil, err
		}
		root = predecessor.root
		diagnostic, err = openSourceReleaseDiagnostic(inputs, outputParents[1], args[3], check)
		if err != nil {
			return nil, err
		}
		if err := predecessor.authenticate(ctx, diagnostic, plan); err != nil {
			return nil, err
		}
	}
	paths := []string{infrastructure.ProgramTarget, infrastructure.GenerationTarget}
	sign := func(value sourceSignableMetadata) ([]byte, error) {
		for _, signer := range signers[:3] {
			if err := check(); err != nil {
				return nil, err
			}
			if _, err := value.Sign(signer); err != nil {
				return nil, err
			}
		}
		raw, err := value.ToBytes(false)
		return raw, errors.Join(err, check())
	}
	var rootBytes []byte
	if predecessor == nil {
		rootBytes, err = sign(root)
		if err != nil {
			return nil, err
		}
	} else {
		rootBytes = bytes.Clone(predecessor.files["root.json"])
	}
	var targetBytes, snapshotBytes []byte
	snapshotVersion := int64(1)
	if purpose == sourceReleaseTimestamp {
		// This purpose signs only Timestamp. Even equivalent reserialization of
		// Root, Snapshot or Targets would destroy the expiry control's cause.
		targetBytes = bytes.Clone(predecessor.files["targets.json"])
		snapshotBytes = bytes.Clone(predecessor.files["snapshot.json"])
		snapshotVersion = predecessor.snapshotVersion
	} else {
		targets := metadata.Targets(plan.Expires)
		if predecessor != nil {
			targets.Signed.Version = predecessor.targetsVersion + 1
		}
		customs := []json.RawMessage{plan.ProgramCustom, plan.GenerationCustom}
		for i, path := range paths {
			digest := sha256.Sum256(artifacts[i])
			custom := json.RawMessage(bytes.Clone(customs[i]))
			targets.Signed.Targets[path] = &metadata.TargetFiles{Length: int64(len(artifacts[i])), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: path, Custom: &custom}
		}
		targetBytes, err = sign(targets)
		if err != nil {
			return nil, err
		}
		snapshot := metadata.Snapshot(plan.Expires)
		snapshot.Signed.Meta["targets.json"] = sourceMetadataReference(targetBytes)
		snapshot.Signed.Meta["targets.json"].Version = targets.Signed.Version
		if predecessor != nil {
			snapshot.Signed.Version = predecessor.snapshotVersion + 1
		}
		snapshotVersion = snapshot.Signed.Version
		snapshotBytes, err = sign(snapshot)
		if err != nil {
			return nil, err
		}
	}
	timestamp := metadata.Timestamp(plan.Expires)
	timestamp.Signed.Meta["snapshot.json"] = sourceMetadataReference(snapshotBytes)
	timestamp.Signed.Meta["snapshot.json"].Version = snapshotVersion
	if predecessor != nil {
		timestamp.Signed.Version = predecessor.timestampVersion + 1
	}
	timestampBytes, err := sign(timestamp)
	if err != nil {
		return nil, err
	}
	if observe != nil {
		observe("signed", keyDirectory.Descriptor())
	}
	if err := check(); err != nil {
		return nil, err
	}
	// The diagnostic history is exclusively born and never reused on failure.
	v := diagnostic
	if v == nil {
		v, err = openSourceReleaseDiagnostic(inputs, outputParents[1], args[3], check)
		if err != nil {
			return nil, err
		}
	}
	metadataFiles, err := release.StaticMetadataFiles(rootBytes, timestampBytes, snapshotBytes, targetBytes)
	if err == nil {
		var proofs []release.Authorization
		for i, path := range paths {
			if err = check(); err != nil {
				break
			}
			decision := v.Evaluate(ctx, release.Inputs{RootBytes: rootBytes, Files: metadataFiles, TargetPath: path, Artifact: artifacts[i], Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: plan.Network, RefTime: plan.Reference.UTC()}})
			proof, ok := decision.Authorization()
			if !ok {
				err = fmt.Errorf("source metadata self-verification refused: %s: %s", decision.Outcome, decision.Notice)
				break
			}
			proofs = append(proofs, proof)
		}
		if err == nil {
			err = v.CheckAuthorizations(ctx, proofs...)
		}
	}
	err = errors.Join(err, v.Close(), check())
	diagnostic = nil
	if err != nil {
		return nil, err
	}
	files := []artifactio.Content{{Name: "root.json", Data: rootBytes}, {Name: "timestamp.json", Data: timestampBytes}, {Name: "snapshot.json", Data: snapshotBytes}, {Name: "targets.json", Data: targetBytes}}
	if err := artifactio.Publish(ctx, args[2], files, check, observe); err != nil {
		return nil, err
	}
	return bytes.Clone(rootBytes), check()
}

func openSourceReleaseDiagnostic(inputs *artifactio.Inputs, parent *artifactio.File, path string, check func() error) (*release.Verifier, error) {
	if err := check(); err != nil {
		return nil, err
	}
	if err := syscall.Mkdirat(int(parent.Descriptor().Fd()), filepath.Base(path), 0700); err != nil {
		return nil, err
	}
	directory, err := artifactio.OpenChild(parent, filepath.Base(path), true)
	if err != nil {
		return nil, err
	}
	if _, err := inputs.Retain(directory, parent, filepath.Base(path)); err != nil {
		return nil, err
	}
	if err := errors.Join(parent.Descriptor().Sync(), check()); err != nil {
		return nil, err
	}
	return release.Open(path)
}

type sourceSignableMetadata interface {
	Sign(signature.Signer) (*metadata.Signature, error)
	ToBytes(bool) ([]byte, error)
}

func sourceMetadataReference(raw []byte) *metadata.MetaFiles {
	digest := sha256.Sum256(raw)
	return &metadata.MetaFiles{Version: 1, Length: int64(len(raw)), Hashes: metadata.Hashes{"sha256": digest[:]}}
}
