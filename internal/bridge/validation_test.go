package bridge_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestInviteEveryTruncationAndMutationFailsClosed(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	defer owner.Close()
	valid := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	for length := 0; length < len(valid); length++ {
		result, err := owner.Import(valid[:length])
		if err != nil || result.Class == "accepted" || result.Class == "already-present" {
			t.Fatalf("truncation %d accepted: %+v, %v", length, result, err)
		}
	}
	for offset := range valid {
		mutated := bytes.Clone(valid)
		mutated[offset] ^= 1
		result, err := owner.Import(mutated)
		if err != nil || result.Class == "accepted" || result.Class == "already-present" {
			t.Fatalf("mutation %d accepted: %+v, %v", offset, result, err)
		}
	}
}

func TestImportRejectsInsufficientTimeConfidenceWithoutDurableMutation(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	config := fixture.config()
	config.TimeConfidence = func() bool { return false }
	owner, err := bridge.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	before := durableFiles(t, fixture.root)
	result, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter))
	if err != nil || result.Class != "incompatible" {
		t.Fatalf("insufficient Time Confidence = %+v, %v", result, err)
	}
	if after := durableFiles(t, fixture.root); !reflect.DeepEqual(before, after) {
		t.Fatal("insufficient Time Confidence changed durable Bridge state")
	}
}

func TestOpenDoesNotClaimAnUnrelatedDirectory(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	if err := os.MkdirAll(fixture.root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fixture.root, "keep.txt")
	if err := os.WriteFile(path, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Open(fixture.config()); err == nil {
		t.Fatal("unrelated directory was claimed as Bridge state")
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "unrelated" {
		t.Fatalf("unrelated directory changed: %q, %v", raw, err)
	}
	if entries, err := os.ReadDir(fixture.root); err != nil || len(entries) != 1 || entries[0].Name() != "keep.txt" {
		t.Fatalf("unrelated directory entries changed: %v, %v", entries, err)
	}
}

func TestOwnerRefreshesTimeNetworkAndLocalRoleFactsPerImport(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		change func(*state.Snapshot, *time.Time, *bool)
		want   string
	}{
		"expired clock": {change: func(_ *state.Snapshot, now *time.Time, _ *bool) {
			*now = now.Add(2 * time.Hour)
		}, want: "expired"},
		"successor epoch": {change: func(snapshot *state.Snapshot, _ *time.Time, _ *bool) {
			snapshot.Epoch++
		}, want: "incompatible"},
		"new local conflict": {change: func(_ *state.Snapshot, _ *time.Time, conflict *bool) {
			*conflict = true
		}, want: "conflicting-role"},
	}
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := newFixture(t)
			snapshot, now, conflict := fixture.snapshot, fixture.now, false
			config := fixture.config()
			config.CurrentNetwork = func() (state.Snapshot, error) { return snapshot, nil }
			config.Clock = func() time.Time { return now }
			config.RoleConflict = func([32]byte, [32]byte) (bool, error) { return conflict, nil }
			owner, err := bridge.Open(config)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			test.change(&snapshot, &now, &conflict)
			result, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter))
			if err != nil || string(result.Class) != test.want {
				t.Fatalf("refreshed import = %+v, %v, want %s", result, err, test.want)
			}
		})
	}
}

func TestOwnerRequiresReopenAfterCurrentPublicationFailure(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	current := filepath.Join(fixture.root, "current")
	if err := os.Mkdir(current, 0o700); err != nil {
		t.Fatal(err)
	}
	invite := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	if _, err := owner.Import(invite); err == nil {
		t.Fatal("import succeeded despite an unpublishable current pointer")
	}
	if _, err := owner.Import(fixture.inviteFor(t, 1, 1, 1, nil, fixture.notBefore, fixture.notAfter)); err == nil {
		t.Fatal("owner remained usable after partial durable publication")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(current); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	defer owner.Close()
	if result, err := owner.Import(invite); err != nil || result.Class != "already-present" {
		t.Fatalf("recovered publication = %+v, %v", result, err)
	}
}

func TestUnknownInviteSchemaIsIncompatible(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	defer owner.Close()
	fields := fixture.fields(0, 1, nil, fixture.notBefore, fixture.notAfter)
	raw := fixture.encode(t, fields)
	raw[len("ardents-h3-bi1")+2+1] = 2
	result, err := owner.Import(raw)
	if err != nil || result.Class != "incompatible" {
		t.Fatalf("unknown version = %+v, %v", result, err)
	}
}

func TestOwnerRequiresFreshConflictFreeStateAndCandidateValidation(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		configure func(*fixture)
		want      string
	}{
		"uncertain time": {
			configure: func(f *fixture) { f.snapshot.Freshness = "clock-uncertain" }, want: "incompatible",
		},
		"conflicting role": {
			configure: func(f *fixture) { f.snapshot.Conflicting = true }, want: "conflicting-role",
		},
	}
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := newFixture(t)
			test.configure(&fixture)
			owner := fixture.open(t)
			defer owner.Close()
			result, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter))
			if err != nil || string(result.Class) != test.want {
				t.Fatalf("Import() = %+v, %v, want %s", result, err, test.want)
			}
		})
	}

	fixture := newFixture(t)
	config := fixture.config()
	config.ValidateCandidate = func([]byte, [32]byte) ([32]byte, string, error) {
		return [32]byte{}, "", errors.New("rejected candidate")
	}
	owner, err := bridge.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if result, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil || result.Class != "invalid" {
		t.Fatalf("candidate rejection = %+v, %v", result, err)
	}
}

func TestOwnerRecoversWatermarkPublishedBeforeCurrentPointer(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	invite := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	owner := fixture.open(t)
	if result, err := owner.Import(invite); err != nil || result.Class != "accepted" {
		t.Fatalf("initial import = %+v, %v", result, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fixture.root, "current")); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	defer owner.Close()
	if result, err := owner.Import(invite); err != nil || result.Class != "already-present" {
		t.Fatalf("watermark-ahead recovery = %+v, %v", result, err)
	}
}

func TestOwnerRejectsSlotAndGenerationBoundaryViolations(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	defer owner.Close()
	invalid := []inviteFields{
		fixture.fieldsFor(0, 2, 1, nil, fixture.notBefore, fixture.notAfter),
		fixture.fieldsFor(0, 0, 0, nil, fixture.notBefore, fixture.notAfter),
	}
	for index, fields := range invalid {
		result, err := owner.Import(fixture.encode(t, fields))
		if err != nil || result.Class != "invalid" {
			t.Fatalf("invalid boundary %d = %+v, %v", index, result, err)
		}
	}
	first := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	accepted, err := owner.Import(first)
	if err != nil || accepted.Class != "accepted" {
		t.Fatalf("initial import = %+v, %v", accepted, err)
	}
	wrong := [32]byte{9}
	violations := [][]byte{
		fixture.inviteFor(t, 1, 0, 1, nil, fixture.notBefore, fixture.notAfter),
		fixture.inviteFor(t, 1, 0, 1, &wrong, fixture.notBefore, fixture.notAfter),
		fixture.inviteFor(t, 1, 0, 2, &wrong, fixture.notBefore, fixture.notAfter),
	}
	for index, invite := range violations {
		result, err := owner.Import(invite)
		if err != nil || result.Class != "replacement-rejected" {
			t.Fatalf("replacement boundary %d = %+v, %v", index, result, err)
		}
	}
}
