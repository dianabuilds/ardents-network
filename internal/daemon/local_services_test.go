package daemon

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	appdata "ardents/internal/content"
	"ardents/internal/discovery"
	records "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	transport "ardents/internal/network"
	networkwaku "ardents/internal/network/waku"
	apppolicy "ardents/internal/policy"
	workloadcontroller "ardents/internal/workload/execution"
	workloadregistry "ardents/internal/workload/registry"
)

func TestReplaceTrustWithLocalPrincipalAddsOnlyDiscoveryPurpose(t *testing.T) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	principalID, err := identityprincipal.FromEd25519PublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := identitytrust.NewRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	evaluator := discovery.NewTrustEvaluator(configured)
	if err := replaceTrustWithLocalPrincipal(evaluator, configured, principalID.String(), base64.StdEncoding.EncodeToString(public)); err != nil {
		t.Fatal(err)
	}
	record := trustSignedRecordForLocalTest(t, private, principalID)
	if result := evaluator.Evaluate(record); !result.Usable {
		t.Fatalf("local discovery result = %#v, want usable", result)
	}
}

func trustSignedRecordForLocalTest(t *testing.T, private ed25519.PrivateKey, principalID identityprincipal.ID) discovery.Record {
	t.Helper()
	now := time.Now().UTC()
	record := discovery.Record{Version: 1, Node: &records.NodeFacts{
		Principal: principalID, PublicKey: base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey)),
	}, IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Hour)}
	payload, err := discovery.Canonical(record)
	if err != nil {
		t.Fatal(err)
	}
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return record
}

func TestConfigureLocalServicesWiresBootstrapAndAdmission(t *testing.T) {
	policy := apppolicy.New(apppolicy.Config{DeniedWorkloadRequirements: []workloadregistry.WorkloadRequirement{"net.admin"}})
	workload := workloadcontroller.NewInDir(t.TempDir())
	data := appdata.NewInDir(t.TempDir())
	trans := networkwaku.New()

	configureLocalServices(policy, workload, data, trans, []string{"local://bootstrap"}, func(transport.BootstrapDialReport) {})

	if err := workload.Register(workloadregistry.Spec{
		ID:           "work-1",
		Kind:         "service",
		Owner:        "node",
		Desired:      workloadregistry.DesiredRunning,
		Requirements: []workloadregistry.WorkloadRequirement{"net.admin"},
	}); err != nil {
		t.Fatalf("register workload: %v", err)
	}
	if err := workload.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile workload: %v", err)
	}
	status, ok := workload.Get("work-1")
	if !ok {
		t.Fatal("expected workload status after reconcile")
	}
	if status.Observed != workloadcontroller.ObservedFailed {
		t.Fatalf("observed = %q, want admission failure", status.Observed)
	}
	if status.Reason == "" {
		t.Fatal("expected admission rejection reason")
	}
}

func TestNormalizeNodeUsesDefaultNameForDataDir(t *testing.T) {
	name, dir := normalizeNode("", "")
	if name != "ardents" {
		t.Fatalf("name = %q, want ardents", name)
	}
	if dir != "var\\ardents" && dir != "var/ardents" {
		t.Fatalf("dir = %q, want var/ardents", dir)
	}
}

func TestNewCoreBuildsBootStatusAndHosting(t *testing.T) {
	core := buildCore(coreConfig{
		Name:    "node-a",
		DataDir: t.TempDir(),
		Boot:    BootConfig{Sources: []string{"local://bootstrap"}},
		Services: []workloadregistry.ServiceSpec{{
			ID:   "svc-a",
			Type: "http",
		}},
	})
	if core.Boot == nil || len(core.Boot.Sources()) != 1 {
		t.Fatalf("boot = %#v, want configured boot status", core.Boot)
	}
	if items := core.Hosting.List(); len(items) != 1 || items[0].ID != "svc-a" {
		t.Fatalf("hosting = %#v, want single configured service", items)
	}
}
