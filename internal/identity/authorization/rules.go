package authorization

import identityapi "ardents/internal/identity/api"

func HasCapability(capabilities []string, domain string, access Access) bool {
	return identityapi.HasCapability(capabilities, domain, access)
}
