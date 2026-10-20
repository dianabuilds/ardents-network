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
package epoch
