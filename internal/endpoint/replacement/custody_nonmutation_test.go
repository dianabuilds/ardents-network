package replacement

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/custody"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestReplacePreservesAuthorityVaultAndReleaseFloors(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	for _, test := range []struct {
		name string
		run  func(*testing.T, protectedReplacementFixture)
	}{
		{name: "success", run: runSuccessfulProtectedReplacement},
		{name: "refusal", run: runRefusedProtectedReplacement},
		{name: "rollback", run: runRollbackProtectedReplacement},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := replacementProtectedFixture(t)
			beforeFloors, err := json.Marshal(fixture.floors)
			if err != nil {
				t.Fatal(err)
			}
			test.run(t, fixture)
			assertProtectedTreeUnchanged(t, fixture.vaultRoot, fixture.vaultBefore)
			assertProtectedTreeUnchanged(t, fixture.releaseRoot, fixture.releaseBefore)
			afterFloors, err := json.Marshal(fixture.floors)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(beforeFloors, afterFloors) {
				t.Fatal("release floors in the authorization changed during replacement")
			}
		})
	}
}

func runSuccessfulProtectedReplacement(t *testing.T, fixture protectedReplacementFixture) {
	t.Helper()
	result, err := Replace(context.Background(), Operation{
		Request: Request{StateRoot: fixture.stateRoot, Artifact: fixture.candidate,
			decision: replacementDecisionWithFloors(fixture.candidate, 2, fixture.floors)},
		ProgramPath: fixture.program, Unit: &replacementUnit{}, SelfTest: replacementSelfTest{program: fixture.program},
	})
	if err != nil || result.State != "committed-restart-permitted" {
		t.Fatalf("Replace() = %+v, %v", result, err)
	}
}

func runRefusedProtectedReplacement(t *testing.T, fixture protectedReplacementFixture) {
	t.Helper()
	result, err := Replace(context.Background(), Operation{
		Request: Request{StateRoot: fixture.stateRoot, Artifact: fixture.candidate,
			decision: replacementDecisionWithFloors(fixture.candidate, 2, fixture.floors)},
		ProgramPath: fixture.program, Unit: refusingReplacementUnit{}, SelfTest: replacementSelfTest{program: fixture.program},
	})
	if err == nil || result.State != "stop-refused" {
		t.Fatalf("refused Replace() = %+v, %v", result, err)
	}
}

func runRollbackProtectedReplacement(t *testing.T, fixture protectedReplacementFixture) {
	t.Helper()
	failed, err := Replace(context.Background(), Operation{
		Request: Request{StateRoot: fixture.stateRoot, Artifact: fixture.candidate,
			decision: replacementDecisionWithFloors(fixture.candidate, 2, fixture.floors)},
		ProgramPath: fixture.program, Unit: &replacementUnit{}, SelfTest: failingSelfTest{},
	})
	if err == nil || failed.State != "rollback-authorization-required" {
		t.Fatalf("failed Replace() = %+v, %v", failed, err)
	}
	result, err := Rollback(context.Background(), Operation{
		Request:     Request{StateRoot: fixture.stateRoot, Artifact: fixture.current, Authorization: fixture.authorization},
		ProgramPath: fixture.program, Unit: &replacementUnit{}, SelfTest: replacementSelfTest{program: fixture.program},
	})
	if err != nil || result.State != "rollback-committed-restart-permitted" {
		t.Fatalf("Rollback() = %+v, %v", result, err)
	}
}

type protectedReplacementFixture struct {
	program, stateRoot, vaultRoot, releaseRoot string
	current, candidate                         []byte
	authorization                              release.Authorization
	floors                                     release.FloorSet
	vaultBefore, releaseBefore                 protectedTree
}

func replacementProtectedFixture(t *testing.T) protectedReplacementFixture {
	t.Helper()
	root := t.TempDir()
	fixture := protectedReplacementFixture{
		program:     filepath.Join(root, "ardents"),
		stateRoot:   filepath.Join(root, "state", "replacement"),
		vaultRoot:   filepath.Join(root, "authority-vault"),
		releaseRoot: filepath.Join(root, "release-floors"),
		candidate:   []byte("candidate program v2"),
	}
	fixture.current, fixture.authorization, fixture.floors = replacementReleaseFixture(t, fixture.releaseRoot)
	if err := replacementCreateVault(t, fixture.vaultRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.program, fixture.current, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), Request{StateRoot: fixture.stateRoot,
		Artifact: fixture.current, Authorization: fixture.authorization}); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPrepared(fixture.stateRoot, fixture.program); err != nil {
		t.Fatal(err)
	}
	fixture.vaultBefore = captureProtectedTree(t, fixture.vaultRoot)
	fixture.releaseBefore = captureProtectedTree(t, fixture.releaseRoot)
	return fixture
}

func replacementReleaseFixture(t *testing.T, root string) ([]byte, release.Authorization, release.FloorSet) {
	t.Helper()
	vector := filepath.Join("..", "..", "release", "testdata", "r049-public-vector-v1")
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(vector, name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	refTime := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	verifier, err := release.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	decision := verifier.Evaluate(context.Background(), release.Inputs{
		RootBytes: read("root.json"),
		Files: map[string][]byte{
			"https://release.invalid/metadata/timestamp.json":  read("timestamp.json"),
			"https://release.invalid/metadata/1.snapshot.json": read("1.snapshot.json"),
			"https://release.invalid/metadata/1.targets.json":  read("1.targets.json"),
		},
		TargetPath: "ardents/windows-amd64/application", Artifact: read("artifact.bin"),
		Local: release.LocalEnvironment{Environment: "h3-test", Network: "ardents-h3-test-1",
			Platform: "windows-amd64", Architecture: "amd64", RefTime: refTime},
	})
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	if decision.Outcome != release.OutcomeReleaseAccepted {
		t.Fatalf("release fixture outcome = %s, want %s", decision.Outcome, release.OutcomeReleaseAccepted)
	}
	authorization, ok := decision.Authorization()
	if !ok {
		t.Fatal("release fixture did not produce an authorization")
	}
	return read("artifact.bin"), authorization, cloneReplacementFloors(decision.Floors)
}

func replacementCreateVault(t *testing.T, root string) error {
	t.Helper()
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		return err
	}
	password := []byte("replacement custody vault password")
	var binding custody.AuthorityBinding
	for index := range binding.Environment {
		binding.Environment[index] = byte(index + 1)
		binding.Network[index] = byte(index + 2)
		binding.Root[index] = byte(index + 3)
		binding.IDCommitment[index] = byte(index + 4)
	}
	binding.Kind = custody.AuthorityService
	state := custody.AuthorityState{Binding: binding, RootMaterial: []byte("replacement authority root material"),
		Generation: 3, Revision: 7, Watermarks: []custody.Watermark{{Domain: "credential-generation", Value: 3}}}
	_, executeErr := vault.Execute(context.Background(), custody.Operation{Kind: custody.OperationCreateVaultRecord,
		Authority: state}, &replacementSecrets{values: [][]byte{password, password}})
	return errors.Join(executeErr, vault.Close())
}

type replacementSecrets struct{ values [][]byte }

type refusingReplacementUnit struct{}

func (refusingReplacementUnit) Stop(context.Context) error {
	return errors.New("replacement unit stop refused")
}

func (refusingReplacementUnit) Start(context.Context) error {
	return errors.New("replacement unit start refused")
}

func (input *replacementSecrets) ReadSecret(context.Context, custody.SecretPrompt) ([]byte, error) {
	if len(input.values) == 0 {
		return nil, errors.New("unexpected custody secret read")
	}
	value := append([]byte(nil), input.values[0]...)
	input.values = input.values[1:]
	return value, nil
}

func (*replacementSecrets) Confirm(context.Context, custody.ConfirmationPrompt) (bool, error) {
	return false, errors.New("unexpected custody confirmation")
}

type protectedTree map[string]protectedEntry

type protectedEntry struct {
	directory bool
	contents  []byte
}

func captureProtectedTree(t *testing.T, root string) protectedTree {
	t.Helper()
	tree := protectedTree{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if entry.IsDir() {
			tree[name] = protectedEntry{directory: true}
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("protected state contains a non-regular entry")
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[name] = protectedEntry{contents: contents}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func assertProtectedTreeUnchanged(t *testing.T, root string, before protectedTree) {
	t.Helper()
	after := captureProtectedTree(t, root)
	if len(after) != len(before) {
		t.Fatalf("protected state entry count changed: before=%d after=%d", len(before), len(after))
	}
	for name, expected := range before {
		observed, ok := after[name]
		if !ok || observed.directory != expected.directory || !bytes.Equal(observed.contents, expected.contents) {
			t.Fatalf("protected state entry %q changed", name)
		}
	}
}

func replacementDecisionWithFloors(artifact []byte, version int64, floors release.FloorSet) release.Decision {
	decision := replacementDecision(artifact, version)
	decision.Floors = cloneReplacementFloors(floors)
	return decision
}

func cloneReplacementFloors(floors release.FloorSet) release.FloorSet {
	return release.FloorSet{RootVersion: floors.RootVersion, RootDigest: append([]byte(nil), floors.RootDigest...),
		TimestampVersion: floors.TimestampVersion, TimestampDigest: append([]byte(nil), floors.TimestampDigest...),
		SnapshotVersion: floors.SnapshotVersion, SnapshotDigest: append([]byte(nil), floors.SnapshotDigest...),
		TargetsVersion: floors.TargetsVersion, TargetsDigest: append([]byte(nil), floors.TargetsDigest...)}
}
