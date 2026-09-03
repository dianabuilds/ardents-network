package endpoint

import nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"

func validateNameOrigin(input connectionInput, credential publicationCredential) error {
	return nativeconnection.ValidateNameOrigin(input.NameBinding, input.NameUpdates, credential.Target,
		input.OpenAttachment != nil, input.RecoveryBinding.DestinationBinding)
}
