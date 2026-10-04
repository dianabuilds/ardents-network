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
package network
