// Package epoch authenticates the bounded Network Epoch and Candidate View
// and returns one immutable verified decision for State admission. It owns
// canonical grammar, signatures, commitments and proofs. Network supplies
// eligibility, collision, family-summary and assignment decisions; this adapter
// does not select Source results or publish State.
// Under the bounded qualification operation it also prepares unsigned fresh
// closed Node Records and initial Epoch commitments. These producers use the
// same grammar and Network evaluation rules; signatures and State acceptance remain
// separate operations.
// On Linux, VerifyInitialClosedRecord additionally verifies an exact signed
// initial Record against its canonical preparation for the offline certificate
// consumer. Detached self-asserted facts grant no State membership or duty.
//
// Responsibility clusters and their tests:
//   - envelope, Record, header and text grammar with canonical truncation
//     refusals: envelope.go, record.go, header.go, text.go
//     (text_test.go, truncation_refusal_test.go)
//   - frozen property and transition evidence for the shared codec and chain
//     bound: envelope.go, record.go, chain.go, assignment.go
//     (verification_property_test.go; testdata/assignment-transition.json,
//     testdata/epoch.hex and testdata/input-0000.hex carry the frozen
//     transition record and the fuzz seeds)
//   - commitment leaves, canonical tree and inclusion proofs: commitment.go,
//     commitment_tree.go, commitment_proof.go (commitment_test.go)
//   - decision verification, candidate evaluation and materialization
//     binding: decision.go, candidate_view.go, candidate_evaluation.go,
//     candidate_projection.go, materialization.go
//     (verification_test.go, candidate_commitment_test.go)
//   - bounded domain selection, role assignment and profile admission:
//     assignment.go, role_assignment.go, profile.go
//     (assignment_test.go, profile_test.go)
//   - historical duty and gateway extension bindings retained on the
//     verified Decision: transit_issuance.go,
//     destination_resolution_gateway.go, materialized_snapshot.go
//     (historical_route_extensions_test.go; built through the shared
//     tests/epochfixture harnesses)
//   - unsigned fresh initial Epoch and Record preparation with exact signed
//     verification: initial_epoch_preparation.go,
//     initial_record_preparation.go, initial_record_verification.go
//     (initial_preparation_test.go, initial_record_verification_test.go)
package epoch
