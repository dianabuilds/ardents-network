// Package custody owns encrypted purpose-specific Authority records, private
// Admission roots, serialized original operations and monotonic recovery floors.
// Admission Allocation alone decides quota, exact retry, journal and hour policy.
// Custody signs only that exact owner decision and acknowledges only after an
// encrypted successor/readback and floor. No private root, generic Sign, Service
// issuer, State/Node/Release signing or restored activation API exists here.
//
// The canonical ardents-authority-envelope-v1 uses fixed Argon2id v1.3 (256 MiB,
// three passes, four lanes) and AES-256-GCM. Generic Service/retired Name records
// remain encrypted evidence. Restores stay in separate export-only quarantine.
// Close denies effects, cancels and joins the original user before lease return.
// Independent development roots do not authorize predecessor history adoption.
// Native mechanisms are development evidence, not platform/crash qualification.
package custody
