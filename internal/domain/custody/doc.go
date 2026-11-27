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
// package-level behavior tests named by behavior, not by source mirror, and
// each test is listed under its primary responsibility cluster.
//
// Responsibility clusters and their tests:
//   - vault lifecycle, Execute dispatch and original operation: vault.go,
//     vault_contract.go, vault_lock.go and its platform implementations
//     (original_operation_test.go, vault_lock_windows_test.go)
//   - encrypted record naming, bounded I/O, no-replace publication and
//     readback: vault_record.go and durability platform implementations
//     (record_refusal_test.go, filesystem_failure_linux_test.go,
//     service_failure_linux_test.go)
//   - Authority record creation, verification and active/quarantine lookup:
//     authority_record.go; confirmed removal: vault_purge.go
//     (recovery_lifecycle_test.go)
//   - canonical envelope and KDF: canonical_envelope.go
//     (envelope_vector_test.go, admission_kdf_test.go)
//   - authority state and recovery floors: authority_state.go,
//     authority_floor.go (floor rules), record_comparison.go
//     (admission_recovery_test.go, interrupted_publication_test.go)
//   - admission purpose: admission_authority.go (admission_authority_test.go,
//     admission_allocation_test.go, admission_refusal_test.go,
//     issuance_time_test.go)
//   - service purpose: service_authority.go, service_credential.go
//     (service_issuance_test.go, service_refusal_test.go)
//   - recovery bundle export, restore and publication: bundle_export.go,
//     bundle_restore.go (bundle_publication_test.go, recovery_lifecycle_test.go)
//   - independent root provenance: independent_root.go
//     (independent_root_refusal_test.go)
//   - post-commit credential delivery: credential_delivery.go and
//     private_permission_linux.go/private_permission_other.go. Linux admits
//     owner-only ext4/tmpfs files; other platforms refuse private delivery
//     before staging (credential_delivery_linux_test.go,
//     credential_delivery_other_test.go). Public responses remain separate.
//
// terminal_input_test.go carries the shared sequenceSecrets input fixture and
// no behavior of its own.
package custody
