package update

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type schemaContractProbe struct{ calls uint64 }

func (probe *schemaContractProbe) Plan(context.Context, uint64, SchemaSelection) (SchemaSelection, bool, error) {
	probe.calls++
	return SchemaSelection{}, false, errors.New("unexpected schema plan")
}

func (probe *schemaContractProbe) Prepare(context.Context, SchemaSelection) error {
	probe.calls++
	return errors.New("unexpected schema prepare")
}

func (probe *schemaContractProbe) Inspect(context.Context, SchemaSelection) error {
	probe.calls++
	return errors.New("unexpected schema inspect")
}

func (probe *schemaContractProbe) Discard(context.Context, SchemaSelection) error {
	probe.calls++
	return errors.New("unexpected schema discard")
}

func TestCOWMissingRootDoesNotInvokeAdapter(t *testing.T) {
	vector := oracleLoadV0(t)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	probe := &schemaContractProbe{}
	request := Request{UpdateRoot: filepath.Join(t.TempDir(), "not-created"), generation: 1,
		schemaPlan: "copy-on-write-v1", decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}, Schema: probe}

	result, err := Apply(context.Background(), request)
	if err == nil || result.Outcome != invalidOutcome || result.State != "release-accepted" || probe.calls != 0 {
		t.Fatalf("Apply = %+v, %v; schema calls=%d", result, err, probe.calls)
	}
}

func TestNoOpSchemaDoesNotInvokeAdapter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	probe := &schemaContractProbe{}
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "no-op-v1",
		decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{},
		SelfTest: oraclePassSelfTest{}, Schema: probe}

	if _, err := Apply(context.Background(), request); err != nil {
		t.Fatalf("Apply no-op schema: %v", err)
	}
	if probe.calls != 0 {
		t.Fatalf("no-op invoked schema adapter %d times", probe.calls)
	}
}

type schemaCOWProbe struct {
	previous, candidate SchemaSelection
	planCalls           uint64
	prepareCalls        uint64
	inspectCalls        uint64
	discardCalls        uint64
	prepared            bool
}

func (probe *schemaCOWProbe) Plan(_ context.Context, generation uint64, previous SchemaSelection) (SchemaSelection, bool, error) {
	probe.planCalls++
	if generation != 1 || previous != probe.previous {
		return SchemaSelection{}, false, errors.New("unexpected COW plan input")
	}
	return probe.candidate, true, nil
}

func (probe *schemaCOWProbe) Prepare(_ context.Context, candidate SchemaSelection) error {
	probe.prepareCalls++
	if candidate != probe.candidate {
		return errors.New("unexpected COW prepare input")
	}
	probe.prepared = true
	return nil
}

func (probe *schemaCOWProbe) Inspect(_ context.Context, candidate SchemaSelection) error {
	probe.inspectCalls++
	if candidate != probe.candidate || !probe.prepared {
		return errors.New("unexpected COW inspect input")
	}
	return nil
}

func (probe *schemaCOWProbe) Discard(_ context.Context, candidate SchemaSelection) error {
	probe.discardCalls++
	if candidate != probe.candidate {
		return errors.New("unexpected COW discard input")
	}
	probe.prepared = false
	return nil
}

func TestCOWCommit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	owner := sha256.Sum256([]byte("schema COW owner"))
	previousContent := sha256.Sum256([]byte("schema COW previous"))
	previous := SchemaSelection{Owner: owner, Content: previousContent}
	previous.Identity = schemaSelectionIdentity(previous)
	previousRaw, err := encodeSchemaCurrent(schemaCurrent{Selection: previous})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema-current"), previousRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	candidateContent := sha256.Sum256([]byte("schema COW candidate"))
	candidateSchema := SchemaSelection{Owner: owner, Generation: 1, Content: candidateContent, Bytes: 5, Entries: 1}
	candidateSchema.Identity = schemaSelectionIdentity(candidateSchema)
	probe := &schemaCOWProbe{previous: previous, candidate: candidateSchema}
	selfTest := &retrySelfTest{}
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "copy-on-write-v1",
		decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{},
		SelfTest: selfTest, Schema: probe}

	first, firstErr := Apply(context.Background(), request)
	if !errors.Is(firstErr, ErrSelfTestUnavailable) || first.Outcome != "application-networking-unverified" || probe.planCalls != 1 ||
		probe.prepareCalls != 1 || probe.inspectCalls != 1 {
		t.Fatalf("first Apply = %+v, %v; schema=%+v", first, firstErr, probe)
	}
	result, err := Apply(context.Background(), request)
	if err != nil || result.Outcome != "committed" || probe.planCalls != 2 || probe.prepareCalls != 1 ||
		probe.inspectCalls != 2 || probe.discardCalls != 0 || selfTest.calls != 2 {
		t.Fatalf("second Apply = %+v, %v; schema=%+v", result, err, probe)
	}
	raw, readErr := os.ReadFile(filepath.Join(root, "schema-current"))
	current, decodeErr := decodeSchemaCurrent(raw)
	if readErr != nil || decodeErr != nil || current.Transaction != 1 || current.Selection != candidateSchema ||
		current.Predecessor != sha256.Sum256(previousRaw) {
		t.Fatalf("schema current=%+v read=%v decode=%v", current, readErr, decodeErr)
	}
	recovered, recoverErr := Recover(context.Background(), root)
	if recoverErr != nil || recovered != result {
		t.Fatalf("Recover = %+v, %v; want %+v", recovered, recoverErr, result)
	}
}

func TestCOWPrepFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	owner := sha256.Sum256([]byte("schema failure owner"))
	content := sha256.Sum256([]byte("schema failure content"))
	previous := SchemaSelection{Owner: owner, Content: content}
	previous.Identity = schemaSelectionIdentity(previous)
	previousRaw, err := encodeSchemaCurrent(schemaCurrent{Selection: previous})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema-current"), previousRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	candidateSchema := previous
	candidateSchema.Generation = 1
	candidateSchema.Content = sha256.Sum256([]byte("schema failure candidate"))
	candidateSchema.Identity = schemaSelectionIdentity(candidateSchema)
	probe := &schemaCOWProbe{previous: previous, candidate: candidateSchema}
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "copy-on-write-v1",
		decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{},
		SelfTest: oraclePassSelfTest{}, Schema: failingSchemaPrepare{schemaCOWProbe: probe}}

	result, applyErr := Apply(context.Background(), request)
	if applyErr == nil || result.Outcome != "staging-failed" || result.State != "draining" || probe.discardCalls != 1 {
		t.Fatalf("Apply = %+v, %v; schema=%+v", result, applyErr, probe)
	}
	if raw, readErr := os.ReadFile(filepath.Join(root, "schema-current")); readErr != nil || string(raw) != string(previousRaw) {
		t.Fatalf("schema selection changed after prepare failure: %x %v", raw, readErr)
	}
}

func TestCOWRollbackDiscardsCandidate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	owner := sha256.Sum256([]byte("schema rollback owner"))
	previousContent := sha256.Sum256([]byte("schema rollback previous"))
	previous := SchemaSelection{Owner: owner, Content: previousContent}
	previous.Identity = schemaSelectionIdentity(previous)
	previousRaw, err := encodeSchemaCurrent(schemaCurrent{Selection: previous})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema-current"), previousRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	candidateContent := sha256.Sum256([]byte("schema rollback candidate"))
	candidateSchema := SchemaSelection{Owner: owner, Generation: 1, Content: candidateContent, Bytes: 9, Entries: 1}
	candidateSchema.Identity = schemaSelectionIdentity(candidateSchema)
	probe := &schemaCOWProbe{previous: previous, candidate: candidateSchema}
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "copy-on-write-v1",
		decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{},
		SelfTest: failedSelfTest{}, Schema: probe}
	if result, applyErr := Apply(context.Background(), request); applyErr == nil || result.Outcome != "self-test-failed" {
		t.Fatalf("forward Apply = %+v, %v", result, applyErr)
	}
	request.rollbackDecision = oracleRollbackDecision(t, vector)
	request.SelfTest = oraclePassSelfTest{}
	result, applyErr := Apply(context.Background(), request)
	if !errors.Is(applyErr, errRolledBack) || result.Outcome != "rolled-back" || probe.planCalls != 2 ||
		probe.prepareCalls != 1 || probe.inspectCalls != 1 || probe.discardCalls != 1 {
		t.Fatalf("rollback Apply = %+v, %v; schema=%+v", result, applyErr, probe)
	}
	raw, readErr := os.ReadFile(filepath.Join(root, "schema-current"))
	if readErr != nil || string(raw) != string(previousRaw) {
		t.Fatalf("schema selection after rollback=%x %v", raw, readErr)
	}
}

func TestCOWTempRecovery(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	owner := sha256.Sum256([]byte("schema temp owner"))
	previousContent := sha256.Sum256([]byte("schema temp previous"))
	previous := SchemaSelection{Owner: owner, Content: previousContent}
	previous.Identity = schemaSelectionIdentity(previous)
	previousRaw, err := encodeSchemaCurrent(schemaCurrent{Selection: previous})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema-current"), previousRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	candidateContent := sha256.Sum256([]byte("schema temp candidate"))
	candidateSchema := SchemaSelection{Owner: owner, Generation: 1, Content: candidateContent, Bytes: 4, Entries: 1}
	candidateSchema.Identity = schemaSelectionIdentity(candidateSchema)
	probe := &schemaCOWProbe{previous: previous, candidate: candidateSchema}
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "copy-on-write-v1",
		decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{},
		SelfTest: &retrySelfTest{}, Schema: probe}
	if _, applyErr := Apply(context.Background(), request); !errors.Is(applyErr, ErrSelfTestUnavailable) {
		t.Fatalf("forward Apply = %v", applyErr)
	}
	journalPath := filepath.Join(root, "transactions", "1", "journal", "08-self-testing.entry")
	journalRaw, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := decodeJournalEntry(journalRaw)
	if err != nil {
		t.Fatal(err)
	}
	entry.AdapterResult = adapterSuccess
	journalRaw, err = encodeJournalEntry(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, journalRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	temporary, err := encodeSchemaCurrent(schemaCurrent{Transaction: 1, Selection: candidateSchema, Predecessor: sha256.Sum256(previousRaw)})
	if err != nil {
		t.Fatal(err)
	}
	tempPath := filepath.Join(root, ".schema-current.0123456789abcdef.tmp")
	if err := os.WriteFile(tempPath, temporary, 0o600); err != nil {
		t.Fatal(err)
	}
	result, recoverErr := Recover(context.Background(), root)
	if recoverErr != nil || result.Outcome != "recovered" || result.State != "self-testing" {
		t.Fatalf("Recover = %+v, %v", result, recoverErr)
	}
	if _, statErr := os.Lstat(tempPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("schema temp remains after recovery: %v", statErr)
	}
}

type failingSchemaPrepare struct{ *schemaCOWProbe }

func (probe failingSchemaPrepare) Prepare(context.Context, SchemaSelection) error {
	probe.prepareCalls++
	probe.prepared = true
	return errors.New("schema prepare failed")
}
