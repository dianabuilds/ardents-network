package authorization

import identityapi "ardents/internal/identity/api"

type Access = identityapi.Access

const (
	AccessRead  = identityapi.AccessRead
	AccessWrite = identityapi.AccessWrite
)

type Subject = identityapi.Subject

type Decision = identityapi.Decision
