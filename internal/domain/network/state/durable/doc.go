// Package durable owns the exclusive Network State root and its physical
// generation, distribution-journal, and closed-profile transactions.
// It stores bounded opaque bytes; state verifies their meaning.
// The selected Linux OpenRetained consumer requires existing complete semantic
// structure and never initializes a missing root or control history.
//
// Responsibility clusters and their tests:
//   - exclusive root opening, lease lifecycle and generation recovery:
//     root.go, filesystem_unix.go, filesystem_windows.go, generation.go
//     (root_test.go, root_symlink_unix_test.go; unix recovery never follows
//     a symlinked generations directory; the platform-local harnesses
//     root_release_failure_unix_test.go; unix and
//     root_release_failure_windows_test.go; windows supply the
//     physically-invalid lease to the Close retention case)
//   - fresh opening refuses every previous attempt: root.go (OpenFresh)
//     (fresh_open_test.go)
//   - retained opening and read-only layout verification: retained_open.go,
//     retained_reader.go (retained_open_test.go,
//     retained_lease_unix_test.go; unix and retained_lease_windows_test.go;
//     windows — the retained lease refuses symlink/reparse-point and
//     hard-link identity and never creates a missing lock)
//   - generation and control journal transactions with owned-staging
//     pruning: generation.go, distribution_journal.go, file_transaction.go
//     (distribution_journal_test.go, file_transaction_test.go)
//   - closed-profile two-record transaction and sync-boundary uncertainty:
//     closed_profile.go (closed_profile_test.go)
//   - Linux retained inspection over the same root contract:
//     inspection_linux.go is exercised by the Node/Source State and
//     admission/spending inspection suites in their owning packages
package durable

// Limits are the State verifier's framing bounds applied at the physical root.
type Limits struct {
	EpochBytes         int64
	RecordBytes        int64
	ClosedProfileBytes int64
}
