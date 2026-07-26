package admission

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"

	"github.com/stretchr/testify/require"
)

func TestDelegationPresentationBoundsMultiplicityAndCanonicalEncoding(t *testing.T) {
	raw := bytes.Repeat([]byte{0xa5}, 32)
	encoded := base64.RawURLEncoding.EncodeToString(raw)

	parsed, err := parseDelegation(http.Header{"ardents-delegation": []string{encoded}})
	require.NoError(t, err)
	require.Equal(t, raw, parsed)
	clear(parsed)

	for name, header := range map[string]http.Header{
		"empty":               {applicationDelegationHeader: []string{""}},
		"padded":              {applicationDelegationHeader: []string{encoded + "="}},
		"whitespace":          {applicationDelegationHeader: []string{" " + encoded}},
		"invalid alphabet":    {applicationDelegationHeader: []string{"opaque-secret-proof!"}},
		"noncanonical bits":   {applicationDelegationHeader: []string{"AB"}},
		"duplicate values":    {applicationDelegationHeader: []string{encoded, encoded}},
		"case-fold duplicate": {applicationDelegationHeader: []string{encoded}, "ardents-delegation": []string{encoded}},
		"encoded oversized":   {applicationDelegationHeader: []string{strings.Repeat("A", base64.RawURLEncoding.EncodedLen(identitycontract.MaxArtifactBytes)+1)}},
	} {
		t.Run(name, func(t *testing.T) {
			presentation, parseErr := parseDelegation(header)
			require.ErrorIs(t, parseErr, identityaccess.ErrUnauthenticated)
			require.Nil(t, presentation)
		})
	}

	tooLarge := bytes.Repeat([]byte{0x5a}, identitycontract.MaxArtifactBytes+1)
	presentation, err := parseDelegation(http.Header{applicationDelegationHeader: []string{base64.RawURLEncoding.EncodeToString(tooLarge)}})
	require.ErrorIs(t, err, identityaccess.ErrUnauthenticated)
	require.Nil(t, presentation)
}
