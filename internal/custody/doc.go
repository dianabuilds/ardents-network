// Package custody owns encrypted purpose-specific Authority records, private
// Admission and Service roots, serialized original operations and recovery floors.
// Admission Allocation alone decides quota, exact retry, journal and hour policy.
// Custody signs only that exact owner decision and acknowledges only after an
// encrypted successor/readback and floor. Service issuance binds separately
// approved raw public requests, non-overlapping generations and terminal bounds.
// No private root, generic Sign, State/Node/Release signing or restored activation
// API exists here.
//
// The canonical ardents-authority-envelope-v1 uses fixed Argon2id v1.3 (256 MiB,
// three passes, four lanes) and AES-256-GCM. Predecessor Service/retired Name
// records remain evidence. Restores stay in separate export-only quarantine.
// Close denies effects, cancels and joins the original user before lease return.
// Independent development roots do not authorize predecessor history adoption.
// Native mechanisms are development evidence, not platform/crash qualification.
//
// Navigation: one Vault owns every cluster below under a single serialized
// original operation. README.md carries the extended owner contract. Test
// naming follows docs/development/repository-layout.md; all tests are
// package-level behavior tests named by behavior, not by source mirror, and a
// test listed under several clusters verifies each of them.
//
//   - C1 vault-lifecycle: vault.go, vault_contract.go, vault_operation.go
//     (Execute dispatch), vault_lock.go, vault_lock_nonwindows.go,
//     vault_lock_windows.go, vault_purge.go. Tests: original_operation_test.go,
//     record_refusal_test.go, terminal_input_test.go, recovery_lifecycle_test.go.
//   - C2 record-durability mechanisms: writeRecordIn, verifyPersistedEnvelope,
//     reserveRecordSpace and readRegularFile inside vault_operation.go;
//     writeAtomicPrivate inside authority_floor.go; durability_nonwindows.go,
//     durability_windows.go. Tests: interrupted_publication_test.go,
//     filesystem_failure_linux_test.go, service_failure_linux_test.go,
//     bundle_publication_test.go.
//   - C3 envelope: canonical_envelope.go, public_envelope_inspection.go.
//     Tests: envelope_vector_test.go, admission_kdf_test.go.
//   - C4 authority-state: authority_state.go, authority_floor.go (floor
//     rules), record_comparison.go. Tests: admission_recovery_test.go,
//     admission_refusal_test.go, service_refusal_test.go,
//     recovery_lifecycle_test.go.
//   - C5 admission-purpose: admission_authority.go. Tests:
//     admission_authority_test.go, admission_allocation_test.go,
//     admission_kdf_test.go, admission_recovery_test.go,
//     admission_refusal_test.go, issuance_time_test.go,
//     interrupted_publication_test.go, filesystem_failure_linux_test.go.
//   - C6 service-purpose: service_authority.go, service_credential.go. Tests:
//     service_issuance_test.go, service_refusal_test.go,
//     service_failure_linux_test.go, record_refusal_test.go.
//   - C7 recovery-bundle: bundle_export.go, bundle_restore.go. Tests:
//     bundle_publication_test.go, recovery_lifecycle_test.go,
//     service_refusal_test.go.
//   - C8 root-provenance: independent_root.go. Tests:
//     independent_root_refusal_test.go, independent_root_refusal_linux_test.go.
package custody
