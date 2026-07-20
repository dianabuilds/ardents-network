package subject

import identityapi "ardents/internal/identity/api"

func NormalizeCapabilities(primary []string, legacy []string) []string {
	return identityapi.NormalizeCapabilities(primary, legacy)
}
