package sessionclient

import (
	"context"
	"errors"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"

	"github.com/stretchr/testify/require"
)

func TestSessionHandshakeValidatesEachResponseAtReceiptTime(t *testing.T) {
	beginReceivedAt := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	completeReceivedAt := beginReceivedAt.Add(7 * time.Second)
	current := beginReceivedAt.Add(-time.Second)
	var challengeValidatedAt time.Time
	var completionValidatedAt time.Time

	handshake := SessionHandshake[string, string, string, string]{
		Now: func() time.Time { return current },
		Begin: func(context.Context) (string, error) {
			current = beginReceivedAt
			return "challenge-wire", nil
		},
		AcceptChallenge: func(wire string, receivedAt time.Time) (string, error) {
			challengeValidatedAt = receivedAt
			return "challenge", nil
		},
		Complete: func(context.Context, string) (string, error) {
			current = completeReceivedAt
			return "completion-wire", nil
		},
		AcceptCompletion: func(wire string, receivedAt time.Time) (string, error) {
			completionValidatedAt = receivedAt
			return "session", nil
		},
	}

	session, err := handshake.Run(context.Background())

	require.NoError(t, err)
	require.Equal(t, "session", session)
	require.Equal(t, beginReceivedAt, challengeValidatedAt)
	require.Equal(t, completeReceivedAt, completionValidatedAt)
}

func TestSessionCompletionContractBoundaries(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	valid := SessionCompletion{
		SessionSecret:   make([]byte, identitycontract.SessionSecretBytes),
		SessionID:       "s1_aaaqeayeaudaocajbifqydiob4ibceqtcqkrmfyydenbwha5dypq",
		ExpiresAt:       now.Add(time.Minute),
		EnrollmentProof: nil,
	}
	require.True(t, ValidSessionCompletion(valid, now))

	tests := map[string]func(*SessionCompletion){
		"short secret": func(value *SessionCompletion) {
			value.SessionSecret = value.SessionSecret[:identitycontract.SessionSecretBytes-1]
		},
		"zero session ID": func(value *SessionCompletion) {
			value.SessionID = "s1_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"uppercase ID": func(value *SessionCompletion) {
			value.SessionID = "s1_AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPQ"
		},
		"enrollment proof":   func(value *SessionCompletion) { value.EnrollmentProof = []byte{1} },
		"expired at receipt": func(value *SessionCompletion) { value.ExpiresAt = now },
		"over max lifetime": func(value *SessionCompletion) {
			value.ExpiresAt = now.Add(identitycontract.MaxSessionLifetime + time.Second)
		},
		"fractional expiry": func(value *SessionCompletion) { value.ExpiresAt = value.ExpiresAt.Add(time.Nanosecond) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := valid
			mutate(&value)
			require.False(t, ValidSessionCompletion(value, now))
		})
	}

	maximum := valid
	maximum.ExpiresAt = now.Add(identitycontract.MaxSessionLifetime)
	require.True(t, ValidSessionCompletion(maximum, now))
	require.False(t, ValidSessionCompletion(maximum, now.Add(-time.Nanosecond)))
}

func TestPrincipalSessionChallengeTimeMatrixHasNoSkewExtension(t *testing.T) {
	issuedAt := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	expiresAt := issuedAt.Add(identitycontract.ChallengeLifetime)
	tests := []struct {
		name       string
		receivedAt time.Time
		valid      bool
	}{
		{name: "same second", receivedAt: issuedAt, valid: true},
		{name: "response crosses second boundary", receivedAt: issuedAt.Add(1500 * time.Millisecond), valid: true},
		{name: "client behind by one nanosecond", receivedAt: issuedAt.Add(-time.Nanosecond), valid: false},
		{name: "immediately before expiry", receivedAt: expiresAt.Add(-time.Nanosecond), valid: true},
		{name: "at expiry", receivedAt: expiresAt, valid: false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.valid, ValidAuthenticationChallengeTimes(issuedAt, expiresAt, testCase.receivedAt))
		})
	}

	require.False(t, ValidAuthenticationChallengeTimes(issuedAt.Add(time.Nanosecond), expiresAt, issuedAt))
	require.False(t, ValidAuthenticationChallengeTimes(issuedAt, expiresAt.Add(time.Second), issuedAt))
}

func TestPrincipalSessionFailureRetryMatrix(t *testing.T) {
	live := context.Background()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	require.True(t, RetrySessionAuthentication(live, context.Canceled))
	require.True(t, RetrySessionAuthentication(live, context.DeadlineExceeded))
	require.False(t, RetrySessionAuthentication(canceled, context.Canceled))
	require.False(t, RetrySessionAuthentication(live, errors.New("remote authentication failed")))
}
