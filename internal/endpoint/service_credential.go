package endpoint

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// Credential is the Authority-signed public delegation owned by publication.
// The alias preserves this temporary adapter's product vocabulary while M9
// replaces the former serviceconn owner.
type Credential = publication.Credential

func validateCredential(value Credential, authority, network [32]byte, at time.Time, capability uint32) error {
	return publication.Validate(value, authority, network, at, capability)
}
