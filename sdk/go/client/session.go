package client

import (
	"context"

	sdkidentity "ardents/sdk/go/identity"
	"ardents/sdk/go/internal/adapter"
)

// SessionSigner is intentionally typed. It cannot be used as an opaque signing
// oracle and never exposes the Principal root key.
type SessionSigner interface {
	Principal(context.Context) (string, error)
	Credential(context.Context) (*sdkidentity.Artifact, error)
	SignAuthenticationChallenge(context.Context, sdkidentity.Challenge) ([]byte, error)
}

// SessionStatus contains no session identifier or secret.
type SessionStatus struct {
	Authenticated   bool
	NodePrincipal   string
	SignerPrincipal string
}

// SessionProvider controls the process-local session cache. Logout invalidates
// cached secrets in this process; server-side revocation is checked on calls.
type SessionProvider interface {
	Authenticate(context.Context) error
	Status() SessionStatus
	Logout()
}

type sessionProvider struct{ manager *adapter.SessionManager }

func (p *sessionProvider) Authenticate(ctx context.Context) error {
	return p.manager.Authenticate(ctx)
}

func (p *sessionProvider) Status() SessionStatus {
	status := p.manager.Status()
	return SessionStatus{
		Authenticated: status.Authenticated, NodePrincipal: status.NodePrincipal,
		SignerPrincipal: status.SignerPrincipal,
	}
}

func (p *sessionProvider) Logout() { p.manager.Logout() }
