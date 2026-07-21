// Package identity owns node identity lifecycle and authentication facts.
// It does not own transport or general policy.
package identity

func Authorize(call CallContext, domain string, access Access) Decision {
	return AuthorizeSubject(NormalizeSubject(call), domain, access)
}

func AuthorizeAction(call CallContext, action string, access Access) Decision {
	return AuthorizeSubjectAction(NormalizeSubject(call), action, access)
}

func AuthorizeSubjectAction(subject Subject, action string, access Access) Decision {
	if !subject.Authenticated {
		return Decision{
			Code:    "unauthorized",
			Message: "authentication required",
			Reason:  "authenticated subject required",
		}
	}
	if HasActionCapability(subject.Capabilities, action) {
		return Decision{Allowed: true}
	}
	return Decision{
		Code:    "forbidden",
		Message: string(access) + " action capability required",
		Reason:  "missing capability " + action,
	}
}

func AuthorizeSubject(subject Subject, domain string, access Access) Decision {
	if !subject.Authenticated {
		return Decision{
			Code:    "unauthorized",
			Message: "authentication required",
			Reason:  "authenticated subject required",
		}
	}
	if HasCapability(subject.Capabilities, domain, access) {
		return Decision{Allowed: true}
	}
	return Decision{
		Code:    "forbidden",
		Message: string(access) + " capability required",
		Reason:  "missing capability " + domain + ":" + string(access),
	}
}

func HasCapability(capabilities []string, domain string, access Access) bool {
	required := domain + ":" + string(access)
	domainWildcard := domain + ":*"
	for _, capability := range capabilities {
		switch capability {
		case "*", "admin", domainWildcard, required:
			return true
		}
	}
	return false
}

func HasActionCapability(capabilities []string, action string) bool {
	for _, capability := range capabilities {
		if capability == "*" || capability == "admin" || capability == action {
			return true
		}
	}
	return false
}
