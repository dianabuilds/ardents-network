package subject

import identityapi "ardents/internal/identity/api"

func NormalizeCall(call identityapi.CallContext) Subject {
	return identityapi.NormalizeSubject(call)
}
