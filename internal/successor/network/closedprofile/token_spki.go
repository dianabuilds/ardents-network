package closedprofile

import "github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"

// ValidateTokenSPKI delegates the selected canonical RSA-PSS public grammar to
// Admission. This format adapter binds public inventory, never private issuance
// material or current Network authority.
func ValidateTokenSPKI(encoded []byte) bool {
	return issuerprofile.ValidateTokenSPKI(encoded)
}
