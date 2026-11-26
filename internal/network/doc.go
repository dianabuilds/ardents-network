// Package network owns decisions about accepted network generations and their
// currentness. Authentication supplies facts; persistence commits decisions.
// Candidate View policy owns eligibility, collision exclusion, family totals
// and deterministic Role Domain assignment without authenticating wire bytes.
// Acquisition retains finite contact attempts and original cycle bounds within
// State's commit boundary; it owns no transport or independently mutable root.
// Neither a parsed document nor a transition proposal authorizes network work.
// Global State and installation-local participation are separate consistency
// owners. Transport, physical roots, process resources, token debit and physical
// reservations belong to collaborating application and supporting owners.
//
// Responsibility clusters and their tests:
//   - accepted State reconstruction and retained duty projection:
//     accepted_state.go, profile_facts.go, retained_duty.go
//     (accepted_state_test.go)
//   - finite contact history and exposure bounds: acquisition.go
//     (acquisition_test.go)
//   - membership evidence binding: membership.go, member.go
//     (membership_test.go)
//   - candidate view eligibility, collision exclusion and role-domain
//     assignment: candidate_view.go, role_assignment.go
//     (candidate_view_test.go, role_assignment_test.go)
//   - epoch transition history: epoch_history.go (epoch_history_test.go)
//   - installation-local participation consistency: local_participation.go
//     (local_participation_test.go)
//   - original participation restriction projection: node_participation.go,
//     node_original_participation.go (node_family_test.go)
//   - profile history conflict preservation: profile_history.go
//     (profile_history_test.go)
//   - time confidence and clock evidence: time_confidence.go
//     (time_confidence_test.go)
package network
