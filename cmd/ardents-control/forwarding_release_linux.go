package main

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
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// Forwarding has only this closed initial signing purpose. It never enters
// Source's initial/forward/timestamp dispatcher or authorizes a native effect.
func runForwardingReleaseSigning(args []string, out io.Writer) error {
	if len(args) != 4 {
		return errors.New("usage: ardents-control forwarding sign-initial-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	root, err := signForwardingRelease(ctx, args, nil)
	if err != nil {
		return &forwardingControlFailure{err}
	}
	sum := sha256.Sum256(root)
	raw, err := json.Marshal(struct {
		Operation   string   `json:"operation"`
		Root        string   `json:"root_sha256"`
		Targets     []string `json:"targets"`
		Installed   bool     `json:"installed_qualification"`
		Independent bool     `json:"independent_custody"`
	}{"forwarding.sign-initial-release", hex.EncodeToString(sum[:]), []string{infrastructure.ProgramTarget, infrastructure.ForwardingGenerationTarget}, false, false})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}
func signForwardingRelease(ctx context.Context, args []string, observe func(string, *os.File)) (result []byte, returned error) {
	const wantedArguments = 4
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) != wantedArguments {
		return nil, errSourceControlInput
	}
	for _, path := range args {
		if !sourceControlPath(path) {
			return nil, errSourceControlInput
		}
	}
	for i, path := range args[1:] {
		for _, previous := range args[1 : i+1] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return nil, errSourceControlInput
			}
		}
	}
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if returned != nil {
			result = nil
		}
	}()
	planRaw, err := inputs.read(args[0], 1<<20)
	if err != nil {
		return nil, err
	}
	plan, err := decodeForwardingReleasePlan(planRaw, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	var artifacts [3][]byte
	for i, path := range []string{plan.Program, plan.Descriptor, plan.Template} {
		for _, root := range args[1:] {
			if path == root || strings.HasPrefix(path, root+"/") {
				return nil, errSourceControlInput
			}
		}
		maximum := int64(64 << 20)
		if i == 1 {
			maximum = 16 << 10
		}
		artifacts[i], err = inputs.read(path, maximum)
		if err != nil {
			return nil, err
		}
	}
	if err := validateForwardingReleaseResources(plan, artifacts[0], artifacts[1], artifacts[2]); err != nil {
		return nil, err
	}
	keyDirectory, err := inputs.directory(args[1])
	if err != nil {
		return nil, err
	}
	if keyDirectory.info.Mode() != os.ModeDir|0700 {
		return nil, errSourceControlInput
	}
	names, err := keyDirectory.file.Readdirnames(6)
	if err != nil && err != io.EOF || len(names) != 5 {
		return nil, errSourceControlInput
	}
	wanted := map[string]bool{}
	for i := 1; i <= 5; i++ {
		wanted[fmt.Sprintf("release-key-%d.pem", i)] = true
	}
	for _, name := range names {
		if !wanted[name] {
			return nil, errSourceControlInput
		}
	}
	// Retain both future-output parent namespaces before private-key use.
	var outputParents [2]*sourceControlFile
	for i, path := range args[2:4] {
		parent, err := inputs.directory(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		if parent.info.Mode() != os.ModeDir|0700 {
			return nil, errSourceControlInput
		}
		var fs syscall.Statfs_t
		if err := syscall.Fstatfs(int(parent.file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
			return nil, errors.Join(errSourceControlInput, err)
		}
		probe, err := sourceControlOpen(parent, filepath.Base(path), false)
		if probe != nil {
			return nil, errors.Join(errSourceControlInput, probe.Close())
		}
		if !errors.Is(err, syscall.ENOENT) {
			return nil, errors.Join(errSourceControlInput, err)
		}
		outputParents[i] = parent
	}
	check := func() error {
		now := time.Now().UTC()
		if plan.Reference.Before(now.Add(-5*time.Minute)) || plan.Reference.After(now.Add(time.Minute)) || !now.Before(plan.Expires) {
			return errSourceControlInput
		}
		if _, err := keyDirectory.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		names, err := keyDirectory.file.Readdirnames(6)
		if err != nil && err != io.EOF || len(names) != 5 {
			return errSourceControlInput
		}
		for _, name := range names {
			if !wanted[name] {
				return errSourceControlInput
			}
		}
		return inputs.checkProtected()
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
		raw, err := inputs.read(filepath.Join(args[1], fmt.Sprintf("release-key-%d.pem", i)), 4096)
		if err != nil {
			return nil, err
		}
		info := inputs.files[len(inputs.files)-1].info
		if info.Mode() != 0600 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			return nil, errSourceControlInput
		}
		key, err := parseSourceSigningKey(raw)
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
			return nil, errSourceControlInput
		}
		root.Signed.Keys[id] = public
		keyIDs = append(keyIDs, id)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), keyIDs...), Threshold: 3}
	}

	paths := []string{infrastructure.ProgramTarget, infrastructure.ForwardingGenerationTarget}
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
	rootBytes, err := sign(root)
	if err != nil {
		return nil, err
	}
	targets := metadata.Targets(plan.Expires)
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
	snapshotBytes, err := sign(snapshot)
	if err != nil {
		return nil, err
	}
	timestamp := metadata.Timestamp(plan.Expires)
	timestamp.Signed.Meta["snapshot.json"] = sourceMetadataReference(snapshotBytes)
	timestampBytes, err := sign(timestamp)
	if err != nil {
		return nil, err
	}
	if observe != nil {
		observe("signed", keyDirectory.file)
	}
	if err := check(); err != nil {
		return nil, err
	}
	// The diagnostic history is exclusively born and never reused on failure.
	v, err := openSourceReleaseDiagnostic(inputs, outputParents[1], args[3], check)
	if err != nil {
		return nil, err
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
				err = fmt.Errorf("forwarding metadata self-verification refused: %s: %s", decision.Outcome, decision.Notice)
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
	files := []sourceArtifactFile{{"root.json", rootBytes}, {"timestamp.json", timestampBytes}, {"snapshot.json", snapshotBytes}, {"targets.json", targetBytes}}
	if err := persistSourceArtifacts(ctx, args[2], files, check, observe); err != nil {
		return nil, err
	}
	return bytes.Clone(rootBytes), check()
}
