package endpoint

import nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"

func validateNameOrigin(input connectionInput, credential publicationCredential) error {
	return nativeconnection.ValidateNameOrigin(input.NameBinding, input.NameUpdates, credential.Target,
		input.OpenAttachment != nil, input.RecoveryBinding.DestinationBinding)
}

// continuesNameBinding is retained only as a behavior-test compatibility name;
// the native connection owner evaluates the continuity rule.
func continuesNameBinding(initial, update destinationBinding) bool {
	return nativeconnection.ContinuesNameOrigin(initial, update)
}
