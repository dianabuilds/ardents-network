package authorization

import identityapi "ardents/internal/identity/api"

func NormalizeCall(call identityapi.CallContext) Subject {
	return identityapi.NormalizeSubject(call)
}

func Require(subject Subject, domain string, access Access) Decision {
	return identityapi.AuthorizeSubject(subject, domain, access)
}
