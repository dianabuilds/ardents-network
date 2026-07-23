package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	sdkidentity "ardents/sdk/go/identity"
	"ardents/sdk/go/internal/adapter"
)

// EnrollmentSigner exposes only the two typed operations required for
// Application enrollment. It is intentionally not a generic signing oracle.
type EnrollmentSigner interface {
	Principal(context.Context) (string, error)
	Credential(context.Context) (*sdkidentity.Artifact, error)
	SignEnrollmentChallenge(context.Context, sdkidentity.Challenge) ([]byte, error)
}

type ApplicationEnrollmentTicket struct {
	value [identitycontract.ApplicationEnrollmentTicketBytes]byte
}

func ParseApplicationEnrollmentTicket(encoded string) (ApplicationEnrollmentTicket, error) {
	var result ApplicationEnrollmentTicket
	raw, ok := identitycontract.DecodeApplicationEnrollmentTicket(encoded)
	if !ok {
		return result, fmt.Errorf("Application enrollment ticket is invalid")
	}
	result.value = raw
	return result, nil
}

func (ApplicationEnrollmentTicket) String() string { return "[redacted Application enrollment ticket]" }
func (ApplicationEnrollmentTicket) GoString() string {
	return "[redacted Application enrollment ticket]"
}
func (ApplicationEnrollmentTicket) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[redacted Application enrollment ticket]"))
}
func (ApplicationEnrollmentTicket) MarshalJSON() ([]byte, error) {
	return json.Marshal("[redacted Application enrollment ticket]")
}

type EnrollmentConfig struct {
	SocketPath    string
	NodePrincipal string
	Ticket        ApplicationEnrollmentTicket
	Signer        EnrollmentSigner
	HTTPClient    *http.Client
}

type EnrollmentResult struct {
	Principal      string
	CredentialID   string
	GrantID        string
	GrantExpiresAt time.Time
}

// EnrollApplication performs the one-use root possession flow over the
// protected Application Unix listener. Key custody remains with Signer.
func EnrollApplication(ctx context.Context, config EnrollmentConfig) (EnrollmentResult, error) {
	if config.Signer == nil || config.Ticket.value == [identitycontract.ApplicationEnrollmentTicketBytes]byte{} || !adapter.ValidPrincipalID(config.NodePrincipal) {
		return EnrollmentResult{}, fmt.Errorf("Application enrollment configuration is invalid")
	}
	httpClient, err := unixHTTPClient(strings.TrimSpace(config.SocketPath), config.HTTPClient)
	if err != nil {
		return EnrollmentResult{}, err
	}
	ticket := config.Ticket.value
	defer clear(ticket[:])
	result, err := adapter.NewEnrollmentClient(httpClient, "http://localhost", config.Signer, config.NodePrincipal, nil).Enroll(ctx, ticket)
	if err != nil {
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{Principal: result.Principal, CredentialID: result.CredentialID, GrantID: result.GrantID, GrantExpiresAt: result.GrantExpiresAt}, nil
}
