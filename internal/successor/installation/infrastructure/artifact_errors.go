package infrastructure

import releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"

// The existing deployment operations retain the same refusal identities.
var (
	ErrInput         = releaseartifact.ErrInput
	ErrBinding       = releaseartifact.ErrBinding
	ErrAuthorization = releaseartifact.ErrAuthorization
)
