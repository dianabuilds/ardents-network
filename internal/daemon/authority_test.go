package daemon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	authorityapi "ardents/internal/authority"
	runtimeconfig "ardents/internal/config"
	diagapi "ardents/internal/diagnostics"
	apppolicy "ardents/internal/policy"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestRealmAuthorityAuditUsesDurableBoundedProjection(t *testing.T) {
	writer := &authorityAuditWriter{}
	record := authorityapi.AuditRecord{
		Version: authorityapi.ContractVersion, ID: "raa1_00112233445566778899aabbccddeeff",
		Actor: "actor-principal", Effective: "actor-principal",
		Action: authorityapi.ActionCreate, ResourceKind: authorityapi.ResourceKindAuthorityInstance,
		ResourceID:  authorityapi.PrimaryAuthorityInstance,
		OperationID: "rao1_00112233445566778899aabbccddeeff",
		Outcome:     "accepted", CreatedAt: time.Date(2026, 7, 27, 15, 0, 0, 0, time.UTC),
	}
	require.NoError(t, (realmAuthorityAudit{events: writer}).RecordAuthorityAudit(context.Background(), record))
	require.Len(t, writer.commands, 1)
	command := writer.commands[0]
	require.Equal(t, "realm_authority", command.Domain)
	require.Equal(t, "primary", command.Resource)
	require.Equal(t, record.Actor, command.Payload["actor"])
	require.Equal(t, record.Effective, command.Payload["effective"])
	require.Equal(t, record.Hash, command.Payload["audit_hash"])
	require.Equal(t, record.PreviousHash, command.Payload["previous_hash"])
	require.Equal(t, record.CreatedAt.Format(time.RFC3339), command.Payload["created_at"])
	raw, err := json.Marshal(command)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private_key")
	require.NotContains(t, string(raw), "checkpoint")
	require.NotContains(t, string(raw), "selector")
}

func TestConfiguredAuthorityReportsMissingExternalSignerWithoutRegeneration(t *testing.T) {
	root := t.TempDir()
	keyPath := filepath.Join(root, "secrets", "store.key")
	require.NoError(t, storage.AtomicWritePrivateFile(
		keyPath, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xa1}, 32))),
	))
	require.NoError(t, storage.EnsurePrivateDir(filepath.Join(root, "authority")))
	checkpointRoot := filepath.Join(root, "independent-checkpoints")
	require.NoError(t, storage.EnsurePrivateDir(checkpointRoot))
	require.NoError(t, storage.AtomicCreatePrivateFile(
		filepath.Join(checkpointRoot, ".ardents-worm-repository-v1.json"),
		[]byte(`{"version":1,"retention":"worm","administration":"independent"}`),
	))
	owners := Owners{RoutePolicy: apppolicy.New(apppolicy.Config{})}
	store := configureRealmAuthority(&owners, runtimeconfig.AuthorityConfig{
		Enabled: true, StorePath: filepath.Join(root, "authority", "realm-authority.db"),
		StoreKeyFile: keyPath, SignerFile: filepath.Join(root, "secrets", "missing-signer.json"),
		CheckpointRepositoryPath: checkpointRoot,
	})
	require.NotNil(t, store)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NotNil(t, owners.Authority)
	require.Equal(t, authorityapi.ReadinessUnavailable, owners.Authority.Readiness().Readiness)
	require.Equal(t, authorityapi.ReasonSignerUnavailable, owners.Authority.Readiness().Reason)
	_, err := owners.Authority.CreateOrReopen(context.Background(), authorityapi.Command{
		Actor: "operator", Effective: "operator", Action: authorityapi.ActionCreate,
		ResourceKind: authorityapi.ResourceKindAuthorityInstance, ResourceID: authorityapi.PrimaryAuthorityInstance,
	}, authorityapi.CreateRequest{
		Version: authorityapi.ContractVersion, RequestID: "request-001",
		RealmClass: authorityapi.RealmClassProduction,
	})
	require.ErrorIs(t, err, authorityapi.ErrUnavailable)
}

type authorityAuditWriter struct{ commands []diagapi.RecordEventCommand }

func (w *authorityAuditWriter) RecordEventCommandDurable(command diagapi.RecordEventCommand) (diagapi.EventEnvelope, error) {
	w.commands = append(w.commands, command)
	return diagapi.EventEnvelope{}, nil
}
