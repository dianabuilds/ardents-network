//go:build linux && amd64 && network_signing_native

package infrastructure

import "path/filepath"

// Retirement has a distinct purpose and journal. The syscall custody oracle
// is shared with startup, but neither observation can satisfy the other.
func (p *sourceRefusalObserverProfile) validRetirementPendingObservation(root string) bool {
	return p != nil && p.EarlyInitialPrefix == 0 && p.RetirementPending != nil && p.StartupPending == nil &&
		p.Schema == "source-retirement-recovery-refusal-observer-v1" &&
		p.RetirementPending.Attempt < uint32(sourceAttemptLimit) && p.RetirementPending.GID >= 100 && p.RetirementPending.GID <= 999 &&
		len(p.Arguments) == 8 && p.Arguments[7] == filepath.Join(sourceAttemptNumber(p.RetirementPending.Attempt).directory(root), "source-retirement-journal") &&
		p.JournalMode == "0600" && p.RecoveryHold == nil && !p.MissingOriginalCut && !p.ExternalJournalLease && !p.ExternalReleaseLease
}

func sourceRetirementPendingNativeCase(scenario string) bool {
	return scenario == "refusal-retirement-pending" || scenario == "refusal-retirement-pending-unattempted" ||
		scenario == "refusal-retirement-pending-foreign" || scenario == "refusal-retirement-pending-follow" || scenario == "refusal-retirement-pending-startup"
}
