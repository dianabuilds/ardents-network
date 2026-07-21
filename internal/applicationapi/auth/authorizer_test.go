package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthorizerSeparatesCredentialsAndCapabilities(t *testing.T) {
	var decisions []Decision
	authorizer, err := New(Config{
		Token: "application-secret", Subject: "example", Capabilities: []string{"application.content.get"},
		Audit: func(decision Decision) { decisions = append(decisions, decision) },
	})
	require.NoError(t, err)

	header := http.Header{"Authorization": []string{"Bearer application-secret"}}
	require.ErrorIs(t, authorizer.Authorize(context.Background(), header, "application.content.get"), ErrUnauthenticated)
	header.Set("Authorization", "ArdentsApplication application-secret")
	require.NoError(t, authorizer.Authorize(context.Background(), header, "application.content.get"))
	require.ErrorIs(t, authorizer.Authorize(context.Background(), header, "application.content.put"), ErrForbidden)
	require.Equal(t, []Decision{
		{Subject: "example", Action: "application.content.get", Outcome: "unauthenticated"},
		{Subject: "example", Action: "application.content.get", Outcome: "allowed"},
		{Subject: "example", Action: "application.content.put", Outcome: "forbidden"},
	}, decisions)
}

func TestAuthorizerRejectsExpiredCredential(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	authorizer, err := New(Config{
		Token: "application-secret", Subject: "example", Capabilities: []string{"application.content.get"},
		ExpiresAt: now, Clock: func() time.Time { return now },
	})
	require.NoError(t, err)
	header := http.Header{"Authorization": []string{"ArdentsApplication application-secret"}}
	require.True(t, errors.Is(authorizer.Authorize(context.Background(), header, "application.content.get"), ErrUnauthenticated))
}
