// Package epoch authenticates the bounded Network Epoch and Candidate View
// and returns one immutable verified decision for State admission. It owns
// canonical grammar, signatures, commitments, assignments, and proofs; it
// does not select Source results or publish State.
// Under the bounded qualification operation it also prepares unsigned fresh
// closed Node Records and initial Epoch commitments. These producers use the
// same grammar and evaluation rules; signatures and State acceptance remain
// separate operations.
package epoch
