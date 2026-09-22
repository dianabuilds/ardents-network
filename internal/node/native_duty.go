package node

import "errors"

func validateNativeDutyProfile(config runtimeConfig, snapshot dutyFacts) error {
	switch snapshot.Assignment {
	case "rendezvous":
		_, err := rendezvousDuty(config.Rendezvous, snapshot)
		return err
	case "transit-issuance":
		return validateTransitIssuerProfile(config.TransitIssuer, snapshot, config.now())
	default:
		return errors.New("native Route assignment is not implemented")
	}
}
