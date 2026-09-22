package node

import "errors"

func validateNativeDutyProfile(config runtimeConfig, snapshot dutyFacts) error {
	switch snapshot.Assignment {
	case "rendezvous":
		_, err := rendezvousDuty(config.Rendezvous, snapshot)
		return err
	case "introduction":
		if snapshot.AuthorityCount == 0 {
			return errors.New("introduction State authority verification set is incomplete")
		}
		_, err := introductionDuty(config.Introduction, snapshot, stateTransitGrantAdmitter(config.LocalRoleStateRoot, snapshot,
			func() (dutyFacts, error) { return snapshot, nil }, config.now))
		return err
	case "responder":
		if snapshot.AuthorityCount == 0 {
			return errors.New("responder State authority verification set is incomplete")
		}
		_, err := responderDuty(config.Responder, snapshot, stateTransitGrantAdmitter(config.LocalRoleStateRoot, snapshot,
			func() (dutyFacts, error) { return snapshot, nil }, config.now))
		return err
	case "transit-issuance":
		return validateTransitIssuerProfile(config.TransitIssuer, snapshot, config.now())
	default:
		return errors.New("native Route assignment is not implemented")
	}
}
