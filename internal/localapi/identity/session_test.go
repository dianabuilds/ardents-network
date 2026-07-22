package identity

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOperatorSessionAcceptsOnlyExactBoundedScheme(t *testing.T) {
	raw := make([]byte, 32)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	value := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(raw)
	secret, err := parseOperatorSession(http.Header{"Authorization": []string{value}})
	require.NoError(t, err)
	require.Equal(t, raw, secret[:])

	for name, values := range map[string][]string{
		"missing":                      nil,
		"bearer does not fall back":    {"Bearer " + strings.Repeat("a", 43)},
		"application session rejected": {"ArdentsApplicationSession " + base64.RawURLEncoding.EncodeToString(raw)},
		"padding rejected":             {value + "="},
		"wrong size":                   {"ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(raw[:31])},
		"multiple":                     {value, value},
		"overlong":                     {"ArdentsOperatorSession " + strings.Repeat("a", 129)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseOperatorSession(http.Header{"Authorization": values})
			require.ErrorIs(t, err, errInvalidSessionHeader)
			require.NotContains(t, err.Error(), base64.RawURLEncoding.EncodeToString(raw))
		})
	}
}
