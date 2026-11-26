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

	"github.com/dianabuilds/ardents-network/internal/domain/release"
	"github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// These fixed initial and forward purposes each bind their role's exact target,
// plan, original key directory and verified predecessor.
func runNodeReleaseCommand(role nodeReleaseRole, args []string, out io.Writer, purpose nodeReleasePurpose) error {
	artifacts, ok := role.artifacts()
	if !ok {
		return artifactio.ErrInput
	}
	wanted, operation := 4, "sign-initial-release"
	if purpose == nodeReleaseForward {
		wanted, operation = 5, "sign-forward-release"
	} else if purpose != nodeReleaseInitial {
		return artifactio.ErrInput
	}
	if len(args) != wanted {
		if purpose == nodeReleaseForward {
			return fmt.Errorf("usage: ardents infrastructure sign-release %s sign-forward-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION PREDECESSOR_DISTRIBUTION", role)
		}
		return fmt.Errorf("usage: ardents infrastructure sign-release %s sign-initial-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION", role)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	root, err := signNodeReleasePurpose(role, ctx, args, purpose, nil)
	if err != nil {
		return nodeReleaseFailure(role, err)
	}
	sum := sha256.Sum256(root)
	raw, err := json.Marshal(struct {
		Operation   string   `json:"operation"`
		Root        string   `json:"root_sha256"`
		Targets     []string `json:"targets"`
		Installed   bool     `json:"installed_qualification"`
		Independent bool     `json:"independent_custody"`
	}{string(role) + "." + operation, hex.EncodeToString(sum[:]), []string{infrastructure.ProgramTarget, artifacts.target}, false, false})
	if err != nil {
		return err
	}
	return artifactio.WriteReceipt(ctx, out, append(raw, '\n'))
}
func nodeReleaseFailure(role nodeReleaseRole, err error) error {
	switch role {
	case forwardingReleaseRole:
		return artifactio.Refuse(err, "forwarding artifact preparation refused")
	case joinReleaseRole:
		return artifactio.Refuse(err, "JOIN artifact preparation refused")
	case descriptorReleaseRole:
		return artifactio.Refuse(err, "Descriptor artifact preparation refused")
	default:
		return artifactio.ErrInput
	}
}

func signNodeReleasePurpose(role nodeReleaseRole, ctx context.Context, args []string, purpose nodeReleasePurpose, observe func(string, *os.File)) (result []byte, returned error) {
	namesForRole, ok := role.artifacts()
	if !ok {
		return nil, artifactio.ErrInput
	}
	wantedArguments := 4
	if purpose == nodeReleaseForward {
		wantedArguments = 5
	} else if purpose != nodeReleaseInitial {
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
	plan, err := decodeNodeReleasePlan(role, planRaw, time.Now().UTC(), purpose)
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
	candidateVersion, err := validateNodeReleaseResources(role, plan, artifacts[0], artifacts[1], artifacts[2])
	if err != nil {
		return nil, err
	}
	var predecessor *nodeReleasePredecessor
	if purpose == nodeReleaseForward {
		predecessor, err = readNodeReleasePredecessor(ctx, role, inputs, args[4], plan, candidateVersion)
		if err != nil {
			return nil, err
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
		if predecessor != nil {
			if err := predecessor.check(); err != nil {
				return err
			}
			if err := predecessor.candidate.Check(); err != nil {
				return err
			}
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
		diagnostic, err = openSourceReleaseDiagnostic(inputs, outputParents[1], args[3], check)
		if err != nil {
			return nil, err
		}
		if err := predecessor.authenticate(ctx, diagnostic, plan); err != nil {
			return nil, err
		}
		root = predecessor.root
	}

	paths := []string{infrastructure.ProgramTarget, namesForRole.target}
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
	targets := metadata.Targets(plan.Expires)
	if predecessor != nil {
		targets.Signed.Version = predecessor.targetsVersion + 1
	}
	for i, path := range paths {
		custom := []json.RawMessage{plan.ProgramCustom, plan.GenerationCustom}[i]
		sum := sha256.Sum256(artifacts[i])
		copyCustom := json.RawMessage(bytes.Clone(custom))
		targets.Signed.Targets[path] = &metadata.TargetFiles{Length: int64(len(artifacts[i])), Hashes: metadata.Hashes{"sha256": sum[:]}, Path: path, Custom: &copyCustom}
	}
	targetBytes, err := sign(targets)
	if err != nil {
		return nil, err
	}
	snapshot := metadata.Snapshot(plan.Expires)
	snapshot.Signed.Meta["targets.json"] = sourceMetadataReference(targetBytes)
	snapshot.Signed.Meta["targets.json"].Version = targets.Signed.Version
	if predecessor != nil {
		snapshot.Signed.Version = predecessor.snapshotVersion + 1
	}
	snapshotBytes, err := sign(snapshot)
	if err != nil {
		return nil, err
	}
	timestamp := metadata.Timestamp(plan.Expires)
	timestamp.Signed.Meta["snapshot.json"] = sourceMetadataReference(snapshotBytes)
	timestamp.Signed.Meta["snapshot.json"].Version = snapshot.Signed.Version
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
	defer func() { returned = errors.Join(returned, v.Close()) }()
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
				err = fmt.Errorf("%s metadata self-verification refused: %s: %s", role, decision.Outcome, decision.Notice)
				break
			}
			proofs = append(proofs, proof)
		}
		if err == nil {
			err = v.CheckAuthorizations(ctx, proofs...)
		}
	}
	err = errors.Join(err, v.Close(), check())
	if err != nil {
		return nil, err
	}
	files := []artifactio.Content{{Name: "root.json", Data: rootBytes}, {Name: "timestamp.json", Data: timestampBytes}, {Name: "snapshot.json", Data: snapshotBytes}, {Name: "targets.json", Data: targetBytes}}
	if err := artifactio.Publish(ctx, args[2], files, check, observe); err != nil {
		return nil, err
	}
	return bytes.Clone(rootBytes), check()
}
