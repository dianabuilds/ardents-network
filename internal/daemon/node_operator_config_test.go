package daemon

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"
	"ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	hostingservice "ardents/internal/workload/registry"

	"github.com/stretchr/testify/require"
)

func TestNodeReloadAppliesPolicyDiagnosticsAndRefreshInterval(t *testing.T) {
	doc := runtimeconfig.Defaults()
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocument(t, path, doc)
	manager, err := runtimeconfig.NewManager(path, doc)
	require.NoError(t, err)
	node := NewNode(Config{
		Name: "ardents", Data: DataConfig{Dir: t.TempDir()},
		Transport:      TransportConfig{BindAddress: "0.0.0.0"},
		OperatorConfig: manager,
	})
	require.NoError(t, node.policy.AllowServicePublication(hostingservice.ServiceSpec{ID: "service-a"}))

	doc.Policy.DisableServicePublication = true
	doc.Diagnostics.MaxEvents = 100
	doc.Network.DiscoveryRefreshSeconds = 5
	writeOperatorDocument(t, path, doc)
	result := node.ReloadConfig(context.Background())

	require.Equal(t, runtimeconfig.OutcomeApplied, result.Outcome)
	require.Error(t, node.policy.AllowServicePublication(hostingservice.ServiceSpec{ID: "service-a"}))
	require.Equal(t, 5*time.Second, node.cfg.DiscoveryRefreshInterval)
	require.Equal(t, uint64(2), node.GetEffectiveConfig().ActiveGeneration)
}

func TestNodeReloadReplacesDiscoveryTrustAndPreservesLocalPublisher(t *testing.T) {
	firstRecord, firstTrust := trustedReloadRecord(t, "first")
	secondRecord, secondTrust := trustedReloadRecord(t, "second")
	doc := runtimeconfig.Defaults()
	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{firstTrust}
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocument(t, path, doc)
	manager, err := runtimeconfig.NewManager(path, doc)
	require.NoError(t, err)
	configured, err := operatorTrustRegistry(doc.Trust)
	require.NoError(t, err)
	node := NewNode(Config{
		Name: "ardents", Data: DataConfig{Dir: t.TempDir()},
		Trust: TrustConfig{Registry: configured}, OperatorConfig: manager,
	})
	localSummary, localPrivate, err := node.ident.EnsureNode(node.state, node.keys)
	require.NoError(t, err)
	require.NoError(t, replaceTrustWithLocalPrincipal(
		node.trust, configured, localSummary.Principal, localSummary.PublicKey,
	))
	localPrincipal, err := identityprincipal.Parse(localSummary.Principal)
	require.NoError(t, err)
	localRecord := trustSignedRecordForLocalTest(t, localPrivate, localPrincipal)
	require.True(t, node.trust.Evaluate(firstRecord).Usable)
	require.False(t, node.trust.Evaluate(secondRecord).Trusted)
	require.True(t, node.trust.Evaluate(localRecord).Usable)

	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{secondTrust}
	writeOperatorDocument(t, path, doc)
	result := node.ReloadConfig(context.Background())

	require.Equal(t, runtimeconfig.OutcomeApplied, result.Outcome)
	require.False(t, node.trust.Evaluate(firstRecord).Trusted)
	require.True(t, node.trust.Evaluate(secondRecord).Usable)
	require.True(t, node.trust.Evaluate(localRecord).Usable)
	require.Equal(t, uint64(2), node.GetEffectiveConfig().ActiveGeneration)
}

func TestNodeReloadRollbackRestoresCompactedDiscoveryTruth(t *testing.T) {
	record, trustConfig := trustedReloadRecord(t, "rollback")
	doc := runtimeconfig.Defaults()
	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{trustConfig}
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocument(t, path, doc)
	manager, err := runtimeconfig.NewManager(path, doc)
	require.NoError(t, err)
	configured, err := operatorTrustRegistry(doc.Trust)
	require.NoError(t, err)
	dataDir := t.TempDir()
	node := NewNode(Config{
		Name: "ardents", Data: DataConfig{Dir: dataDir},
		Trust: TrustConfig{Registry: configured}, OperatorConfig: manager,
	})
	result, err := node.disco.Import(record, discoveryrecord.Bootstrap)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NoError(t, manager.RegisterApplier(&configFailureApplier{failApply: true}))
	doc.Trust.Principals = nil
	writeOperatorDocument(t, path, doc)

	reload := node.ReloadConfig(context.Background())

	require.Equal(t, runtimeconfig.OutcomeRolledBack, reload.Outcome)
	require.True(t, node.trust.Evaluate(record).Usable)
	require.Len(t, node.disco.Entries(), 1)
	reloaded := discovery.NewInDirWithTrust(dataDir, discovery.NewTrustEvaluator(configured))
	require.NoError(t, reloaded.Load())
	require.Len(t, reloaded.Entries(), 1)
}

func TestNodeReloadRollbackPreservesConcurrentDiscoveryImport(t *testing.T) {
	record, trustConfig := trustedReloadRecord(t, "rollback-current")
	concurrent, _ := trustedReloadRecord(t, "rollback-concurrent")
	doc := runtimeconfig.Defaults()
	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{trustConfig}
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocument(t, path, doc)
	manager, err := runtimeconfig.NewManager(path, doc)
	require.NoError(t, err)
	configured, err := operatorTrustRegistry(doc.Trust)
	require.NoError(t, err)
	dataDir := t.TempDir()
	node := NewNode(Config{
		Name: "ardents", Data: DataConfig{Dir: dataDir},
		Trust: TrustConfig{Registry: configured}, OperatorConfig: manager,
	})
	require.NoError(t, node.life.Move("starting"))
	require.NoError(t, node.life.Move("initializing"))
	require.NoError(t, node.life.Move("ready"))
	result, err := node.disco.Import(record, discoveryrecord.Bootstrap)
	require.NoError(t, err)
	require.True(t, result.Applied)
	blocker := &blockingConfigFailureApplier{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	require.NoError(t, manager.RegisterApplier(blocker))
	doc.Trust.Principals = nil
	writeOperatorDocument(t, path, doc)

	reloadResult := make(chan runtimeconfig.ReloadResult, 1)
	go func() {
		reloadResult <- node.ReloadConfig(context.Background())
	}()
	<-blocker.entered
	importResult := make(chan error, 1)
	go func() {
		snapshot := discovery.RecordSnapshot(discovery.Entry{
			Record: concurrent,
			Source: discoveryrecord.Imported,
			SeenAt: time.Now().UTC(),
		})
		_, importErr := node.ImportRecord(snapshot)
		importResult <- importErr
	}()
	importCompleted := false
	select {
	case err := <-importResult:
		require.NoError(t, err)
		importCompleted = true
	case <-time.After(50 * time.Millisecond):
	}
	require.False(t, importCompleted, "Discovery import must wait for config commit or rollback")
	close(blocker.release)

	reload := <-reloadResult
	require.Equal(t, runtimeconfig.OutcomeRolledBack, reload.Outcome)
	if !importCompleted {
		require.NoError(t, <-importResult)
	}
	require.True(t, hasDiscoveryRecord(node.disco.Entries(), concurrent.RecordID()))
	reloaded := discovery.NewInDirWithTrust(dataDir, discovery.NewTrustEvaluator(configured))
	require.NoError(t, reloaded.Load())
	require.True(t, hasDiscoveryRecord(reloaded.Entries(), concurrent.RecordID()))
}

func TestNodePolicyReloadDoesNotRewriteDiscoveryTruth(t *testing.T) {
	record, trustConfig := trustedReloadRecord(t, "policy-only")
	doc := runtimeconfig.Defaults()
	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{trustConfig}
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocument(t, path, doc)
	manager, err := runtimeconfig.NewManager(path, doc)
	require.NoError(t, err)
	configured, err := operatorTrustRegistry(doc.Trust)
	require.NoError(t, err)
	dataDir := t.TempDir()
	node := NewNode(Config{
		Name: "ardents", Data: DataConfig{Dir: dataDir},
		Trust: TrustConfig{Registry: configured}, OperatorConfig: manager,
	})
	result, err := node.disco.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	require.True(t, result.Applied)
	discoveryPath := discovery.PathInDir(dataDir)
	before, err := os.ReadFile(discoveryPath)
	require.NoError(t, err)
	beforeHash := sha256.Sum256(before)

	doc.Policy.DisableServicePublication = true
	writeOperatorDocument(t, path, doc)
	reload := node.ReloadConfig(context.Background())

	require.Equal(t, runtimeconfig.OutcomeApplied, reload.Outcome)
	after, err := os.ReadFile(discoveryPath)
	require.NoError(t, err)
	require.Equal(t, beforeHash, sha256.Sum256(after))
}

func TestNodeReloadDegradesWhenRollbackCannotRestoreRuntime(t *testing.T) {
	doc := runtimeconfig.Defaults()
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocument(t, path, doc)
	manager, err := runtimeconfig.NewManager(path, doc,
		configFailureApplier{failRollback: true},
		configFailureApplier{failApply: true},
	)
	require.NoError(t, err)
	node := NewNode(Config{Name: "ardents", Data: DataConfig{Dir: t.TempDir()}, OperatorConfig: manager})
	require.NoError(t, node.life.Move("starting"))
	require.NoError(t, node.life.Move("initializing"))
	require.NoError(t, node.life.Move("ready"))

	doc.Policy.DisableServicePublication = true
	writeOperatorDocument(t, path, doc)
	result := node.ReloadConfig(context.Background())

	require.Equal(t, runtimeconfig.OutcomeRollbackFailed, result.Outcome)
	require.Equal(t, "degraded", node.life.State())
	snapshot := node.Snapshot()
	require.True(t, hasSubsystemReason(snapshot.Diag.Health.Subsystems, "configuration", "config.reload.rollback_failed"))
}

func trustedReloadRecord(
	t *testing.T,
	id string,
) (discovery.Record, runtimeconfig.TrustedPrincipalConfig) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	encodedPublic := base64.StdEncoding.EncodeToString(public)
	now := time.Now().UTC()
	record := discovery.Record{
		Version: discoveryrecord.Version,
		Service: &discoveryrecord.ServiceFacts{
			ID: discoveryrecord.ServiceID("svc." + id), Type: "echo", NodePrincipal: principal,
			Workload: discoveryrecord.WorkloadID("work." + id), Mode: "NetworkPublished",
			PublicKey: encodedPublic, Endpoints: []string{"https://10.20.30.40:8443"},
		},
		IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Hour),
	}
	payload, err := discovery.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	clear(private)
	return record, runtimeconfig.TrustedPrincipalConfig{
		Principal: principal.String(), PublicKey: encodedPublic,
		Purposes: []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}
}

type configFailureApplier struct {
	failApply    bool
	failRollback bool
}

type blockingConfigFailureApplier struct {
	entered chan struct{}
	release chan struct{}
}

func (*blockingConfigFailureApplier) Prepare(
	context.Context,
	runtimeconfig.Document,
	runtimeconfig.Document,
) error {
	return nil
}

func (a *blockingConfigFailureApplier) Apply(
	context.Context,
	runtimeconfig.Document,
	runtimeconfig.Document,
) error {
	close(a.entered)
	<-a.release
	return errors.New("apply failed")
}

func (*blockingConfigFailureApplier) Rollback(context.Context, runtimeconfig.Document) error {
	return nil
}

func hasDiscoveryRecord(entries []discovery.Entry, id string) bool {
	for _, entry := range entries {
		if entry.Record.RecordID() == id {
			return true
		}
	}
	return false
}

func (configFailureApplier) Prepare(context.Context, runtimeconfig.Document, runtimeconfig.Document) error {
	return nil
}

func (a configFailureApplier) Apply(context.Context, runtimeconfig.Document, runtimeconfig.Document) error {
	if a.failApply {
		return errors.New("apply failed")
	}
	return nil
}

func (a configFailureApplier) Rollback(context.Context, runtimeconfig.Document) error {
	if a.failRollback {
		return errors.New("rollback failed")
	}
	return nil
}

func writeOperatorDocument(t *testing.T, path string, doc runtimeconfig.Document) {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}
