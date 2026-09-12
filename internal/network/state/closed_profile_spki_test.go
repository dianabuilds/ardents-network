package state

import "testing"

func TestValidateClosedTokenSPKIPreservesExactGrammar(t *testing.T) {
	spki := testClosedProfileSPKI(t)
	if !ValidateClosedTokenSPKI(spki) {
		t.Fatal("rejected selected RSA-PSS SPKI")
	}
	changed := append([]byte(nil), spki...)
	changed[len(changed)-1] ^= 1
	if ValidateClosedTokenSPKI(changed) {
		t.Fatal("accepted changed selected RSA-PSS SPKI")
	}
}
