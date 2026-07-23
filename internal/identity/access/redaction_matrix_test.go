package access

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentityAccessSecretClassesAreRedactedFromFormattingJSONAndErrors(t *testing.T) {
	sessionBytes := bytes.Repeat([]byte{0x31}, 32)
	ticketBytes := bytes.Repeat([]byte{0x32}, 32)
	applicationTicketBytes := bytes.Repeat([]byte{0x33}, 32)
	proofBytes := bytes.Repeat([]byte{0x34}, 32)
	credential := []byte("credential-wire-secret-PIA018")
	delegation := []byte("delegation-wire-secret-PIA018")
	signature := []byte("typed-signature-secret-PIA018")
	nonceBytes := bytes.Repeat([]byte{0x35}, 32)
	var nonce [32]byte
	copy(nonce[:], nonceBytes)

	var session SessionSecret
	copy(session[:], sessionBytes)
	var ticket BootstrapTicket
	copy(ticket[:], ticketBytes)
	var applicationTicket ApplicationEnrollmentTicket
	copy(applicationTicket[:], applicationTicketBytes)
	var proof EnrollmentProof
	copy(proof[:], proofBytes)
	challenge := Challenge{Nonce: nonce}
	attempt := Attempt{SessionSecret: session, Delegation: delegation}
	targetAttempt := TargetAttempt{SessionSecret: session, Delegation: delegation}

	values := []any{
		session, ticket, applicationTicket, proof, challenge,
		CompleteRequest{Credential: credential, Signature: signature},
		CompleteResult{SessionSecret: &session, EnrollmentProof: &proof},
		FirstEnrollmentRequest{Ticket: ticket, Challenge: challenge, Proof: proof, Credential: credential},
		EnrollApplicationRequest{Ticket: applicationTicket, Challenge: challenge, Proof: proof, Credential: credential},
		IssueApplicationEnrollmentTicketRequest{Attempt: attempt},
		attempt, targetAttempt,
	}
	var projections strings.Builder
	for _, value := range values {
		_, _ = fmt.Fprintf(&projections, "%v\n%#v\n", value, value)
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		projections.Write(raw)
		projections.WriteByte('\n')
	}

	f := newServiceFixture(t)
	_, err := f.service.Admit(t.Context(), attempt)
	require.Error(t, err)
	projections.WriteString(err.Error())

	output := projections.String()
	for name, secret := range map[string][]byte{
		"session": sessionBytes, "bootstrap ticket": ticketBytes,
		"application ticket": applicationTicketBytes, "proof": proofBytes,
		"credential": credential, "delegation": delegation, "signature": signature, "nonce": nonceBytes,
	} {
		for encodingName, encoded := range map[string]string{
			"raw": string(secret), "hex": hex.EncodeToString(secret),
			"base64":    base64.StdEncoding.EncodeToString(secret),
			"base64url": base64.RawURLEncoding.EncodeToString(secret),
		} {
			require.NotContains(t, output, encoded, "%s leaked as %s", name, encodingName)
		}
	}
}
