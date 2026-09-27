// Package hosting bounds shared provider-period reservations and their release.
// It also owns class-2 reserve-before-spend policy for forwarding and JOIN.
// The package opens the concrete ledger, shares a bounded sampler across
// handles for one local root, and defers close until all borrowers join.
// Node retains process pressure decisions and selects the local period.
package hosting
