package receiver

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
)

// ValidateCarrier rejects a deferred receiving Carrier before composition
// acquires Admission, Hosting or a listener. The signed member supplies profile.
func ValidateCarrier(profile string) error {
	if transport.CarrierProfile(profile) != transport.ClosedCarrierTCP {
		return errors.New("closed shared carrier profile is unsupported")
	}
	return nil
}
