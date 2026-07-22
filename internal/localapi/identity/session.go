// Package identity adapts Principal identity access to the protected Operator
// Unix-socket Connect surface. It never accepts legacy bearer credentials.
package identity

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	identityaccess "ardents/internal/identity/access"
)

const operatorSessionScheme = "ArdentsOperatorSession"

var errInvalidSessionHeader = errors.New("invalid Operator session authorization")

func parseOperatorSession(header http.Header) (identityaccess.SessionSecret, error) {
	var secret identityaccess.SessionSecret
	values := header.Values("Authorization")
	if len(values) != 1 || len(values[0]) > 128 {
		return secret, errInvalidSessionHeader
	}
	prefix := operatorSessionScheme + " "
	if !strings.HasPrefix(values[0], prefix) || strings.Count(values[0], " ") != 1 {
		return secret, errInvalidSessionHeader
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(values[0], prefix))
	if err != nil || len(raw) != len(secret) {
		return secret, errInvalidSessionHeader
	}
	copy(secret[:], raw)
	return secret, nil
}
