package update

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/release"
)

const (
	rootMarkerName, rootMarker, maximumArtifactBytes      = ".ardents-update-transaction-v2", "ardents-update-transaction-v2\n", 64 << 20
	maximumRecordBytes, maximumIdentityBytes              = 16384, 256
	maximumTargetBytes, maximumNoticeBytes                = 512, 512
	recordHeaderBytes, journalBodyBytes                   = 16, 123
	journalRecordBytes, maximumJournalBytes               = recordHeaderBytes + journalBodyBytes, 4096
	recordManifest, recordCurrent                    byte = 1, 2
	recordPredecessor, recordJournal                 byte = 3, 4
	recordSchemaCurrent, recordRollbackRetire        byte = 5, 6
)

// WorkControl stops new admission and drains accepted work before activation.
type WorkControl interface {
	StopNewWork(context.Context) error
	Drain(context.Context) error
	StopNewAssignments(context.Context) error
	DrainAssignments(context.Context) error
	RejoinOrWithdraw(context.Context) error
}

// SelfTest checks the activated candidate before commit.
type SelfTest interface {
	Check(context.Context, CandidateIdentity) error
}

// SchemaSelection is the bounded opaque fact identifying one schema
// generation owned by a caller-provided SchemaWork Adapter.
type SchemaSelection struct {
	Owner      [32]byte
	Generation uint64
	Identity   [32]byte
	Content    [32]byte
	Bytes      uint64
	Entries    uint64
}

// SchemaWork materializes and verifies an unselected schema generation. Its
// implementation owns all schema-root paths and never receives the update
// root, release floors, or other Module-owned state.
type SchemaWork interface {
	Plan(context.Context, uint64, SchemaSelection) (SchemaSelection, bool, error)
	Prepare(context.Context, SchemaSelection) error
	Inspect(context.Context, SchemaSelection) error
	Discard(context.Context, SchemaSelection) error
}

// CandidateIdentity is the bounded value supplied to SelfTest.
type CandidateIdentity struct {
	Generation   uint64
	TargetPath   string
	Length       int64
	Digest       [32]byte
	Platform     string
	Architecture string
	Environment  string
	Network      string
}

// Request carries one opaque release authorization and caller-owned runtime
// Adapters. Its zero Authorization is invalid: an accepted-looking public
// release.Decision is never sufficient to authorize an update.
type Request struct {
	UpdateRoot    string
	Authorization release.Authorization
	decision      release.Decision
	// RollbackAuthorization is a later Release-issued authorization for the
	// retained predecessor. It is consulted only by rollback-pending recovery.
	RollbackAuthorization release.Authorization
	rollbackDecision      release.Decision
	Artifact              []byte
	Work                  WorkControl
	SelfTest              SelfTest
	Schema                SchemaWork
	// generation is a package-private behavior-test seam. Production Apply
	// derives the successor (or an idempotent replay) from the owned root.
	generation uint64
	// schemaPlan is a package-private behavior-test seam. Production Apply
	// selects the bounded no-op schema transition unless its owner supplies a
	// concrete Schema Adapter.
	schemaPlan string
}

// Result is the bounded terminal transaction result.
type Result struct {
	Outcome        string
	State          string
	Generation     uint64
	CurrentDigest  [32]byte
	RollbackDigest [32]byte
	StagingPresent bool
	SafeNotice     string
}
