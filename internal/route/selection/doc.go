// Package selection owns independently leased Route selection roots.
// Installation Entry pairs retain their existing canonical root and six-hour
// bounds. Each local role has a separate persisted Interior pair and original
// thirty-minute horizon. A fresh Network RuntimeView supplies every live check;
// roots, receipts and copied participant values never grant current authority.
// Retained Interior opening checks its format before acquiring a non-creating
// lease and rechecks under that lease; missing history is never a fresh draw.
// Explicit first preparation creates empty Entry and role histories without a
// draw; the retained installation owner cannot initialize either missing root.
// Context-local Rendezvous choices retain their original set and bound across
// physical leg replacement; failures and new exclusions cannot redraw them.
// Entry-pair eligibility/draw/bounds, retained Leg checks and recipient selection
// are portable rules. entry_set.go owns no filesystem, lease or live authority.
// The Linux installation/selection owner separately owns leased durable roots;
// its platform constraint does not constrain checking an already retained Leg.
//
// Navigation: portable rules compose with the leased durable-root owner.
// Test naming follows docs/development/repository-layout.md; a test listed
// under several clusters verifies each of them.
//
//   - rules: leg.go, entry_set.go, rendezvous.go, introduction_recipient.go,
//     issuer_recipient.go, resolution_recipient.go, submission_recipient.go.
//     Tests: entry_set_test.go, leg_test.go, rendezvous_test.go,
//     resolution_recipient_test.go, submission_recipient_test.go,
//     closed_adjacent_domains_test.go. IntroductionDuty has no dedicated
//     in-package test; it is exercised through route/prefix and cmd
//     composition tests.
//   - durable-root: root.go, atomic_files.go, watermark.go, directory_sync.go,
//     root_lease_unix.go, root_lease_windows.go, root_permissions_unix.go,
//     root_permissions_windows.go. Tests: root_hardening_linux_test.go,
//     closed_set_open_cleanup_linux_test.go; atomic persistence is exercised
//     through the store tests below.
//   - entry-store: closed_sets.go, closed_set_store.go. Tests:
//     closed_sets_test.go, closed_set_open_cleanup_linux_test.go,
//     closed_adjacent_domains_test.go.
//   - interior-store: interior_store.go. Tests: interior_store_test.go,
//     interior_refusal_linux_test.go.
//   - composition: installation.go, selection_owner.go (Borrow returns the
//     serialized Owner). Tests: installation_linux_test.go,
//     installation_history_linux_test.go.
package selection
