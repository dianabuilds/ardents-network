package closedprofile

import "testing"

func TestValidateTokenSPKIPreservesExactGrammar(t *testing.T) {
	spki := testClosedProfileSPKI(t)
	if !ValidateTokenSPKI(spki) {
		t.Fatal("rejected selected RSA-PSS SPKI")
	}
	if ValidateTokenSPKI(spki[:len(spki)-1]) || ValidateTokenSPKI(append(append([]byte(nil), spki...), 0)) {
		t.Fatal("accepted an incorrectly sized token SPKI")
	}
	changed := append([]byte(nil), spki...)
	changed[len(changed)-1] ^= 1
	if ValidateTokenSPKI(changed) {
		t.Fatal("accepted changed selected RSA-PSS SPKI")
	}
}
