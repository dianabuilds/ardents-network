package recipient

import (
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
	"github.com/dianabuilds/ardents-network/internal/domain/publication"
)

func TestUnverifiedPublicationGrantsNoRecipientFacts(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	identity := wire.LogicalIdentity{
		Network: [32]byte{1}, Target: [32]byte{2}, Publication: [32]byte{3}, Profile: [32]byte{4},
		ConnectionNonce: [32]byte{5}, InitiatorBinding: [32]byte{6},
		WorkSafetyNotAfter: now.Add(45 * time.Second).Unix(), WorkSafetyMaximum: now.Add(time.Minute).Unix(),
		NoNewRecoveryAfter: now.Add(30 * time.Second).Unix(),
	}
	facts, err := Derive(publication.Proof{}, identity, [32]byte{7}, now.Add(time.Second), now.Add(time.Minute), now, 1, 1)
	if !errors.Is(err, ErrBinding) || facts != (Facts{}) {
		t.Fatalf("unverified Publication supplied recipient facts: %+v, %v", facts, err)
	}
}
