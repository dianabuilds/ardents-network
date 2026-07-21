package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

const credentialScheme = "ArdentsApplication"

type Config struct {
	Token        string
	Subject      string
	Capabilities []string
	ExpiresAt    time.Time
	Clock        func() time.Time
	Audit        func(Decision)
}

type Decision struct {
	Subject string
	Action  string
	Outcome string
}

type Authorizer struct {
	token        string
	subject      string
	capabilities map[string]struct{}
	expiresAt    time.Time
	clock        func() time.Time
	audit        func(Decision)
}

func New(config Config) (*Authorizer, error) {
	token := strings.TrimSpace(config.Token)
	if token == "" || strings.TrimSpace(config.Subject) == "" {
		return nil, ErrUnauthenticated
	}
	capabilities := make(map[string]struct{}, len(config.Capabilities))
	for _, capability := range config.Capabilities {
		capability = strings.TrimSpace(capability)
		if capability != "" {
			capabilities[capability] = struct{}{}
		}
	}
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Authorizer{
		token: token, subject: strings.TrimSpace(config.Subject), capabilities: capabilities,
		expiresAt: config.ExpiresAt, clock: clock,
		audit: config.Audit,
	}, nil
}

func (a *Authorizer) Authorize(_ context.Context, header http.Header, action string) error {
	presented, ok := applicationToken(header.Get("Authorization"))
	if !ok || subtle.ConstantTimeCompare([]byte(presented), []byte(a.token)) != 1 {
		a.record(action, "unauthenticated")
		return ErrUnauthenticated
	}
	if !a.expiresAt.IsZero() && !a.clock().Before(a.expiresAt) {
		a.record(action, "expired")
		return ErrUnauthenticated
	}
	if _, ok := a.capabilities[action]; !ok {
		a.record(action, "forbidden")
		return ErrForbidden
	}
	a.record(action, "allowed")
	return nil
}

func (a *Authorizer) record(action, outcome string) {
	if a.audit != nil {
		a.audit(Decision{Subject: a.subject, Action: action, Outcome: outcome})
	}
}

func applicationToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || parts[0] != credentialScheme || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return parts[1], true
}
