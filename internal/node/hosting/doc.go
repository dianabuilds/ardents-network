// Package hosting bounds shared provider-period reservations and their release.
// It composes Admission redemption with class-2 reservations for forwarding and JOIN and
// the distinct class-1/3 control envelopes after token verification.
// The package adapts the Hosting domain handle and defers close until all
// borrowers join. Hosting owns accounting and bounded shared observations.
// Node retains process pressure decisions and selects the local period.
package hosting
