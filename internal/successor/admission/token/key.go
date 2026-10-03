package token

import (
	"crypto/rsa"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

func parseClosedTokenPublicKey(spki []byte) (*rsa.PublicKey, error) {
	public, valid := issuerprofile.ParseKey(spki)
	if !valid {
		return nil, errors.New("closed token SPKI is not canonical")
	}
	return public, nil
}
