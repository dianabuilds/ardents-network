package daemon

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

type bootstrapHandoffClock struct{ now time.Time }

func (c *bootstrapHandoffClock) Now() time.Time { return c.now }

func TestBootstrapTicketFileFailureIsRecoveredWithoutWaitingForExpiry(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	ctx := context.Background()
	directory := t.TempDir()
	database, err := storage.OpenIdentityAccess(ctx, directory, identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })

	public, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	node, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	clock := &bootstrapHandoffClock{now: time.Date(2032, 4, 5, 6, 7, 8, 0, time.UTC)}
	service, err := identityaccess.NewService(identityaccess.Config{
		Database: database, Clock: clock, EnableBootstrapTickets: true,
	})
	require.NoError(t, err)

	socketPath := filepath.Join(directory, "operator.sock")
	ticketPath := filepath.Join(directory, "operator-bootstrap-ticket")
	require.NoError(t, os.Mkdir(ticketPath, 0o700))
	require.Error(t, ensureFirstOperatorBootstrapTicket(ctx, service, node.String(), socketPath))
	require.NoError(t, os.Remove(ticketPath))

	require.NoError(t, ensureFirstOperatorBootstrapTicket(ctx, service, node.String(), socketPath))
	delivered, found, err := storage.ReadStrictPrivateFileBounded(ticketPath, 128)
	require.NoError(t, err)
	require.True(t, found)

	restarted, err := identityaccess.NewService(identityaccess.Config{
		Database: database, Clock: clock, EnableBootstrapTickets: true,
	})
	require.NoError(t, err)
	require.NoError(t, ensureFirstOperatorBootstrapTicket(ctx, restarted, node.String(), socketPath))
	afterRestart, found, err := storage.ReadStrictPrivateFileBounded(ticketPath, 128)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, delivered, afterRestart)

	clock.now = clock.now.Add(identitycontract.BootstrapTicketLifetime)
	require.NoError(t, ensureFirstOperatorBootstrapTicket(ctx, restarted, node.String(), socketPath))
	afterExpiry, found, err := storage.ReadStrictPrivateFileBounded(ticketPath, 128)
	require.NoError(t, err)
	require.True(t, found)
	require.NotEqual(t, delivered, afterExpiry)
	for _, encoded := range [][]byte{delivered, afterExpiry} {
		require.NotContains(t, logs.String(), string(encoded))
		decoded, err := base64.RawURLEncoding.DecodeString(string(encoded))
		require.NoError(t, err)
		require.NotContains(t, logs.String(), string(decoded))
	}
}
