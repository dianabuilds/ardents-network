package connection

import "errors"

// ValidateNameOrigin admits either no Name-origin fact or one complete pinned
// binding. When recovery is enabled, the Route destination commitment must be
// that same exact binding commitment.
func ValidateNameOrigin(binding DestinationBinding, updates <-chan DestinationBinding, target [32]byte,
	recoveryEnabled bool, recoveryDestination [32]byte) error {
	if binding == (DestinationBinding{}) {
		if updates != nil {
			return errors.New("name updates exist without resolved provenance")
		}
		return nil
	}
	if binding.Target != target || updates == nil || binding.Name == "" || binding.Generation == 0 || binding.Revision == 0 ||
		binding.RecordDigest == [32]byte{} || binding.Commitment == [32]byte{} {
		return errors.New("resolved Service Name provenance is incomplete")
	}
	if recoveryEnabled && recoveryDestination != binding.Commitment {
		return errors.New("recovery destination does not bind the resolved Service Name")
	}
	return nil
}

// ContinuesNameOrigin accepts only an update that preserves the immutable
// Service target and ancestry while advancing the resolved revision or
// repeating its exact record digest.
func ContinuesNameOrigin(initial, update DestinationBinding) bool {
	if update == (DestinationBinding{}) || update.Name != initial.Name || update.Generation != initial.Generation ||
		update.Revision < initial.Revision || update.Authority != initial.Authority || update.Target != initial.Target ||
		update.ParentName != initial.ParentName || update.ParentGeneration != initial.ParentGeneration {
		return false
	}
	return update.Revision > initial.Revision || update.RecordDigest == initial.RecordDigest
}

// ValidateRecovery admits an absent recovery contract only when no Attachment
// opener was supplied. A present contract must bind the ConnectionContext and
// remain inside the Credential and Work Safety lifetime.
func ValidateRecovery(enabled bool, recovery Recovery, atUnix, credentialNotAfter int64) error {
	if !enabled {
		if recovery != (Recovery{}) {
			return errors.New("recovery binding exists without an attachment opener")
		}
		return nil
	}
	if recovery.CandidateView == [32]byte{} || recovery.IsolationContext == [32]byte{} ||
		recovery.DestinationBinding == [32]byte{} || recovery.RouteProfile != Profile ||
		recovery.WorkSafetyNotAfter <= atUnix || recovery.WorkSafetyMaximum < recovery.WorkSafetyNotAfter ||
		recovery.WorkSafetyMaximum > credentialNotAfter || recovery.NoNewRecoveryAfter <= atUnix ||
		recovery.NoNewRecoveryAfter > recovery.WorkSafetyNotAfter {
		return errors.New("fixed recovery values or finite safety bounds are incomplete")
	}
	return nil
}
