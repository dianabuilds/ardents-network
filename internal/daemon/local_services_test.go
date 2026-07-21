package daemon

import (
	"context"
	"testing"

	appdata "ardents/internal/content"
	"ardents/internal/discovery"
	transport "ardents/internal/network"
	networkwaku "ardents/internal/network/waku"
	apppolicy "ardents/internal/policy"
	workloadcontroller "ardents/internal/workload/execution"
	workloadregistry "ardents/internal/workload/registry"
)

func TestApplyTrustAnchors(t *testing.T) {
	trust := discovery.NewTrustEvaluator()
	applyTrustAnchors(trust, []string{"anchor-a", "anchor-b"})
	anchors := trust.Anchors()
	if len(anchors) != 2 {
		t.Fatalf("anchors = %#v, want 2 anchors", anchors)
	}
}

func TestConfigureLocalServicesWiresBootstrapAndAdmission(t *testing.T) {
	policy := apppolicy.New(apppolicy.Config{DeniedCapabilities: []string{"net.admin"}})
	workload := workloadcontroller.NewInDir(t.TempDir())
	data := appdata.NewInDir(t.TempDir())
	trans := networkwaku.New()

	configureLocalServices(policy, workload, data, trans, []string{"local://bootstrap"}, func(transport.BootstrapDialReport) {})

	if err := workload.Register(workloadregistry.Spec{
		ID:           "work-1",
		Kind:         "service",
		Owner:        "node",
		Desired:      workloadregistry.DesiredRunning,
		Capabilities: []string{"net.admin"},
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
