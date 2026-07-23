package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimeconfig "ardents/internal/config"
	diagapi "ardents/internal/diagnostics"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestDaemonRejectsObsoleteCredentialEnvironmentBeforeStartup(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"ardentsd"}
	t.Cleanup(func() { os.Args = originalArgs })
	t.Setenv("ARDENTS_API_TOKEN", "retired-secret-must-not-leak")

	err := Run(nil, nil, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ARDENTS_API_TOKEN")
	require.NotContains(t, err.Error(), "retired-secret-must-not-leak")
	require.False(t, strings.Contains(strings.ToLower(err.Error()), "bearer"))
}

func TestIdentityAccessAuditProjectsCompleteSafeAuthorizationFacts(t *testing.T) {
	recorder := diagapi.NewInDir(t.TempDir())
	sink := identityAccessAudit{events: recorder}
	require.NoError(t, sink.RecordIdentityAccessDurable(identityaccess.AuditEvent{
		Outcome: "accepted", Reason: "mutation_dispatched",
		Principal: "p1_actor", DeviceID: "d1_device",
		Audience: identityaccess.Audience{Node: "p1_node", Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1},
		Actor:    "p1_actor", Effective: "p1_effective", Action: "node.start",
		GrantIDs: []string{"ag1_grant"}, DelegationID: "dg1_delegation", CorrelationID: "c1_00000000000000010000000000000001",
	}))

	events := recorder.Snapshot().RecentEvents
	require.Len(t, events, 1)
	event := events[0]
	require.Equal(t, "identity_access", event.Domain)
	require.Equal(t, "principal_access_accepted", event.Type)
	require.Equal(t, "c1_00000000000000010000000000000001", event.Payload["correlation_id"])
	require.Equal(t, "node.start", event.Payload["action"])
	require.Equal(t, "p1_actor", event.Payload["actor"])
	require.Equal(t, "p1_effective", event.Payload["effective"])
	require.NotContains(t, event.Payload, "session")
	require.NotContains(t, event.Payload, "credential")
	require.NotContains(t, event.Payload, "proof")
	require.NotContains(t, event.Payload, "ticket")
}

func TestDaemonIdentityAccessOpenFailureReleasesStateLockAndCreatesNoListeners(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state")
	require.NoError(t, storage.EnsurePrivateDir(stateDir))
	require.NoError(t, os.Mkdir(storage.IdentityAccessPathInDir(stateDir), 0o700))
	operatorSocket := filepath.Join(dir, "operator.sock")
	applicationSocket := filepath.Join(dir, "application.sock")
	doc := runtimeconfig.Defaults()
	doc.Node.DataDir = stateDir
	doc.API.SocketPath = operatorSocket
	doc.ApplicationInterface.Enabled = true
	doc.ApplicationInterface.SocketPath = applicationSocket
	configPath := filepath.Join(dir, "operator.json")
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, raw, 0o600))
	t.Setenv(runtimeconfig.OperatorFileEnv, configPath)
	originalArgs := os.Args
	os.Args = []string{"ardentsd"}
	t.Cleanup(func() { os.Args = originalArgs })

	err = Run(nil, nil, nil)

	require.ErrorContains(t, err, "open identity access database")
	require.NoFileExists(t, operatorSocket)
	require.NoFileExists(t, applicationSocket)
	lock, lockErr := storage.AcquireStateDirLock(stateDir)
	require.NoError(t, lockErr, "startup failure must release the state-directory lock")
	require.NoError(t, lock.Close())
}
