package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	identitycontract "ardents/api/ardents/identity/v1"

	"github.com/stretchr/testify/require"
)

func TestApplicationEnrollmentTicketParsingIsCanonicalAndRedacted(t *testing.T) {
	raw := make([]byte, identitycontract.ApplicationEnrollmentTicketBytes)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	ticket, err := ParseApplicationEnrollmentTicket(encoded)
	require.NoError(t, err)
	formatted := fmt.Sprintf("%v %#v %x", ticket, ticket, ticket)
	require.NotContains(t, formatted, encoded)
	require.NotContains(t, formatted, fmt.Sprintf("%x", raw))
	jsonRaw, err := json.Marshal(ticket)
	require.NoError(t, err)
	require.NotContains(t, string(jsonRaw), encoded)

	for _, invalid := range []string{"", " " + encoded, encoded + "=", base64.RawURLEncoding.EncodeToString(make([]byte, len(raw)))} {
		_, err = ParseApplicationEnrollmentTicket(invalid)
		require.Error(t, err)
	}
}

func TestApplicationEnrollmentRejectsPaddedNodePrincipalBeforeTransportSetup(t *testing.T) {
	var value [identitycontract.ApplicationEnrollmentTicketBytes]byte
	value[0] = 1
	ticket := ApplicationEnrollmentTicket{value: value}
	for _, node := range []string{" " + canonicalNodePrincipal, canonicalNodePrincipal + "\n"} {
		_, err := EnrollApplication(context.Background(), EnrollmentConfig{
			SocketPath:    filepath.Join(t.TempDir(), "application.sock"),
			NodePrincipal: node,
			Ticket:        ticket,
			Signer:        &clientSignerStub{},
		})
		require.ErrorContains(t, err, "enrollment configuration is invalid")
	}
}
