package updatetransaction

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

const (
	rootMarker, maximumArtifactBytes              = "ardents-update-transaction-v1\n", 64 << 20
	maximumRecordBytes, maximumIdentityBytes      = 16384, 256
	maximumTargetBytes, maximumNoticeBytes        = 512, 512
	recordHeaderBytes, journalBodyBytes           = 16, 123
	journalRecordBytes, maximumJournalBytes       = recordHeaderBytes + journalBodyBytes, 4096
	recordManifest, recordCurrent            byte = 1, 2
	recordPredecessor, recordJournal         byte = 3, 4
	v0BootstrapManifestHex                        = "54d1f66e06df8e09fd734cccb6cc61b9f4880646ecef9dab8926bf65e5bfea96"
)

// WorkControl stops new admission and drains accepted work before activation.
type WorkControl interface {
	StopNewWork(context.Context) error
	Drain(context.Context) error
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
	Plan(context.Context, uint64, string, SchemaSelection) (SchemaSelection, bool, error)
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

// Request carries one accepted release and caller-owned runtime Adapters.
type Request struct {
	UpdateRoot string
	Generation uint64
	ActiveWork uint64
	SchemaPlan string
	Decision   releasedecision.Decision
	// RollbackDecision is a fresh caller-evaluated decision for the retained
	// predecessor. It is consulted only by a later rollback-pending continuation.
	RollbackDecision releasedecision.Decision
	Artifact         []byte
	Work             WorkControl
	SelfTest         SelfTest
	Schema           SchemaWork
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
	CustodyNotice  string
}
