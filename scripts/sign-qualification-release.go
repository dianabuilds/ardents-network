//go:build ignore

// Command sign-qualification-release signs only the two ADR-0120 initial
// protected Endpoint targets. The public plan contains no private material.
package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"golang.org/x/sys/unix"
)

type releasePlan struct {
	Schema           string          `json:"schema"`
	Network          string          `json:"network"`
	Reference        time.Time       `json:"reference_time"`
	Expires          time.Time       `json:"expires"`
	Endpoint         string          `json:"endpoint"`
	Descriptor       string          `json:"descriptor"`
	ResourceRoot     string          `json:"resource_root"`
	EndpointCustom   json.RawMessage `json:"endpoint_custom"`
	GenerationCustom json.RawMessage `json:"generation_custom"`
}

func main() {
	if err := signRelease(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func signRelease() error {
	if os.Geteuid() != 0 || len(os.Args) != 4 {
		return errors.New("requires Linux root: public-plan private-key-directory new-output-directory")
	}
	keyRoot, output := os.Args[2], os.Args[3]
	if err := secureAncestors(keyRoot); err != nil {
		return err
	}
	if err := secureAncestors(filepath.Dir(output)); err != nil {
		return err
	}
	if !filepath.IsAbs(output) || filepath.Clean(output) != output || output == "/" {
		return errors.New("output path is invalid")
	}
	raw, err := boundedRead(os.Args[1], 1<<20)
	if err != nil {
		return err
	}
	var plan releasePlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&plan) != nil || decoder.Decode(new(any)) != io.EOF || plan.Schema != "ardents-qualification-release-signing-plan-v1" || plan.Network == "" || plan.Reference.IsZero() || !plan.Expires.After(plan.Reference) || plan.Expires.Sub(plan.Reference) > 7*24*time.Hour {
		return errors.New("release plan is invalid")
	}
	now := time.Now().UTC()
	if plan.Reference.Before(now.Add(-5*time.Minute)) || plan.Reference.After(now.Add(time.Minute)) || !plan.Expires.After(now) {
		return errors.New("release plan reference or expiry is not current")
	}
	artifacts := make([][]byte, 2)
	for index, path := range []string{plan.Endpoint, plan.Descriptor} {
		artifacts[index], err = boundedRead(path, 64<<20)
		if err != nil {
			return err
		}
	}
	var identity struct {
		Release string `json:"release_identity"`
		Version int64  `json:"release_version"`
	}
	if json.Unmarshal(plan.EndpointCustom, &identity) != nil || identity.Release == "" || identity.Version != 1 {
		return errors.New("initial release identity or version is invalid")
	}
	var generationIdentity struct {
		Release string `json:"release_identity"`
		Version int64  `json:"release_version"`
	}
	if json.Unmarshal(plan.GenerationCustom, &generationIdentity) != nil || identity != generationIdentity {
		return errors.New("release target identities do not agree")
	}
	var descriptorIdentity struct {
		Version int64 `json:"release_version"`
	}
	if json.Unmarshal(artifacts[1], &descriptorIdentity) != nil || descriptorIdentity.Version != identity.Version {
		return errors.New("protected descriptor version does not match the release")
	}
	var common [2][]byte
	for index, custom := range []json.RawMessage{plan.EndpointCustom, plan.GenerationCustom} {
		var fields map[string]json.RawMessage
		if json.Unmarshal(custom, &fields) != nil {
			return errors.New("release custom fields are invalid")
		}
		delete(fields, "builder_attestations")
		common[index], err = json.Marshal(fields)
		if err != nil {
			return err
		}
	}
	if !bytes.Equal(common[0], common[1]) {
		return errors.New("release target build and policy facts do not agree")
	}
	resources := map[string][]byte{"ardents-linux-amd64": artifacts[0]}
	for _, name := range enrollment.ProtectedResourceNames()[1:] {
		resources[name], err = boundedRead(filepath.Join(plan.ResourceRoot, name), 64<<20)
		if err != nil {
			return err
		}
	}
	if err := enrollment.ValidateProtectedGeneration(artifacts[1], resources, identity.Release); err != nil {
		return err
	}
	root := metadata.Root(plan.Expires)
	root.Signed.UnrecognizedFields = map[string]any{"ardents_schema_version": 1, "ardents_profile": "ardents-h3-release-v1", "ardents_environment": "alpha", "ardents_network": plan.Network}
	var signers []signature.Signer
	var privateKeys []ed25519.PrivateKey
	defer func() {
		for _, key := range privateKeys {
			clear(key)
		}
	}()
	var keyIDs []string
	for index := 1; index <= 5; index++ {
		path := filepath.Join(keyRoot, fmt.Sprintf("release-key-%d.pem", index))
		var stat unix.Stat_t
		if unix.Lstat(path, &stat) != nil || stat.Uid != 0 || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0777 != 0600 {
			return errors.New("private release key ownership or mode is invalid")
		}
		encoded, err := boundedRead(path, 4096)
		if err != nil {
			return errors.New("private release key read failed")
		}
		block, rest := pem.Decode(encoded)
		clear(encoded)
		if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
			return errors.New("private release key format is invalid")
		}
		parsed, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		clear(block.Bytes)
		key, ok := parsed.(ed25519.PrivateKey)
		if parseErr != nil || !ok {
			return errors.New("private release key type is invalid")
		}
		privateKeys = append(privateKeys, key)
		signer, err := signature.LoadSigner(key, crypto.Hash(0))
		if err != nil {
			return errors.New("release signer creation failed")
		}
		signers = append(signers, signer)
		public, err := metadata.KeyFromPublicKey(key.Public())
		if err != nil {
			return err
		}
		id, err := public.ID()
		if err != nil {
			return err
		}
		if _, present := root.Signed.Keys[id]; present {
			return errors.New("release keys are duplicated")
		}
		root.Signed.Keys[id] = public
		keyIDs = append(keyIDs, id)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), keyIDs...), Threshold: 3}
	}
	targets := metadata.Targets(plan.Expires)
	paths := []string{"ardents/linux-amd64/endpoint", "ardents/linux-amd64/protected-endpoint"}
	customs := []json.RawMessage{plan.EndpointCustom, plan.GenerationCustom}
	for index, path := range paths {
		digest := sha256.Sum256(artifacts[index])
		targets.Signed.Targets[path] = &metadata.TargetFiles{Length: int64(len(artifacts[index])), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: path, Custom: &customs[index]}
	}
	rootBytes, err := signedBytes(root, signers)
	if err != nil {
		return err
	}
	targetBytes, err := signedBytes(targets, signers)
	if err != nil {
		return err
	}
	snapshot := metadata.Snapshot(plan.Expires)
	snapshot.Signed.Meta["targets.json"] = metaFile(targetBytes)
	snapshotBytes, err := signedBytes(snapshot, signers)
	if err != nil {
		return err
	}
	timestamp := metadata.Timestamp(plan.Expires)
	timestamp.Signed.Meta["snapshot.json"] = metaFile(snapshotBytes)
	timestampBytes, err := signedBytes(timestamp, signers)
	if err != nil {
		return err
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return errors.New("release output already exists or cannot be created")
	}
	files := map[string][]byte{"https://release.invalid/metadata/timestamp.json": timestampBytes, "https://release.invalid/metadata/1.snapshot.json": snapshotBytes, "https://release.invalid/metadata/1.targets.json": targetBytes}
	for index, path := range paths {
		verifier, err := release.Open(filepath.Join(output, fmt.Sprintf("verification-%d", index)))
		if err != nil {
			return err
		}
		decision := verifier.Evaluate(context.Background(), release.Inputs{RootBytes: rootBytes, Files: files, TargetPath: path, Artifact: artifacts[index], Local: release.LocalEnvironment{Environment: "alpha", Network: plan.Network, Platform: "linux-amd64", Architecture: "amd64", RefTime: plan.Reference}})
		closeErr := verifier.Close()
		if decision.Outcome != release.OutcomeReleaseAccepted {
			return errors.Join(fmt.Errorf("signed target refused: %s", decision.Outcome), closeErr)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	for name, data := range map[string][]byte{"1.root.json": rootBytes, "1.targets.json": targetBytes, "1.snapshot.json": snapshotBytes, "timestamp.json": timestampBytes} {
		if err := writeNewRelease(filepath.Join(output, name), data); err != nil {
			return err
		}
	}
	for _, path := range []string{output, filepath.Dir(output)} {
		dir, err := os.Open(path)
		if err != nil {
			return err
		}
		if err = errors.Join(dir.Sync(), dir.Close()); err != nil {
			return err
		}
	}
	digest := sha256.Sum256(rootBytes)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"schema": "ardents-qualification-release-signing-receipt-v1", "root_sha256": hex.EncodeToString(digest[:]), "targets": paths, "installed_qualification": false})
}

type signableMetadata interface {
	Sign(signature.Signer) (*metadata.Signature, error)
	ToBytes(bool) ([]byte, error)
}

func signedBytes(value signableMetadata, signers []signature.Signer) ([]byte, error) {
	for _, signer := range signers[:3] {
		if _, err := value.Sign(signer); err != nil {
			return nil, errors.New("release metadata signing failed")
		}
	}
	return value.ToBytes(false)
}
func metaFile(raw []byte) *metadata.MetaFiles {
	digest := sha256.Sum256(raw)
	return &metadata.MetaFiles{Version: 1, Length: int64(len(raw)), Hashes: metadata.Hashes{"sha256": digest[:]}}
}
func boundedRead(path string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || limit < 1 {
		return nil, errors.New("release input path or limit is invalid")
	}
	if err := secureAncestors(filepath.Dir(path)); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("release input open refused")
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit {
		closeErr := file.Close()
		return nil, errors.Join(errors.New("release input is not a bounded direct regular file"), err, closeErr)
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if int64(len(raw)) > limit {
		clear(raw)
		return nil, errors.Join(errors.New("release input exceeds its opened-file bound"), readErr, closeErr)
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		clear(raw)
		return nil, err
	}
	return raw, nil
}
func secureAncestors(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("private path is invalid")
	}
	for {
		var stat unix.Stat_t
		if unix.Lstat(path, &stat) != nil || stat.Uid != 0 || stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&0022 != 0 {
			return errors.New("private ancestor must be a root-owned non-writable direct directory")
		}
		if path == "/" {
			return nil
		}
		path = filepath.Dir(path)
	}
}
func writeNewRelease(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, err := file.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return errors.Join(err, file.Sync(), file.Close())
}
