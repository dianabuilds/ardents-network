package issuer

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

// ValidateCurrentAuthority binds an Issuer signer to the observed Network duty.
func ValidateCurrentAuthority(view network.RuntimeView, facts admission.AuthorityFacts, now time.Time, signer [32]byte) error {
	member, err := view.Member(facts.IssuerNodeID, now)
	if err != nil || signer == [32]byte{} || member.PublicKey != signer || member.DutyGeneration != facts.IssuerDutyGeneration || member.RoleDomain != 2 || member.Subrole != 6 {
		return errors.Join(errors.New("network issuer duty is unavailable"), err)
	}
	return nil
}
