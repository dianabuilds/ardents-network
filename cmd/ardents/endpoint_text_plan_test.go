package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestHeadlessTextPlanPreservesProtectedProfile(t *testing.T) {
	plan := headlessTextPlanFixture(t)
	plan.NetworkAuthorities = append([]string{strings.Repeat("09", 32)}, plan.NetworkAuthorities...)
	decoded, err := loadHeadlessRuntimePlan(writeHeadlessTextPlan(t, plan))
	if err != nil {
		t.Fatal(err)
	}
	config, refresh, err := headlessNetworkConfig(decoded, time.Now)
	if err != nil || refresh || config.AcceptedProfile != route.ClosedRouteProfile {
		t.Fatalf("protected Network profile changed: %q, refresh=%v, %v", config.AcceptedProfile, refresh, err)
	}
	if hex.EncodeToString(config.ClosedProfileAuthority) != plan.ClosedProfileAuthority {
		t.Fatal("runtime did not preserve the explicitly selected signer")
	}
	owner, openErr := state.Open(config)
	if openErr != nil {
		t.Fatalf("ordinary text command cannot open its selected State profile: %v", openErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	if decoded.ReaderPermission != plan.ReaderPermission || decoded.PublisherPermission != plan.PublisherPermission || decoded.TextTokenRoot != plan.TextTokenRoot {
		t.Fatal("permission ownership changed during plan decoding")
	}
}

func TestHeadlessTextPlanRejectsIncompleteOrMixedContracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*headlessRuntimePlan)
	}{
		{"missing State signer", func(p *headlessRuntimePlan) { p.ClosedProfileAuthority = "" }},
		{"foreign State signer", func(p *headlessRuntimePlan) { p.ClosedProfileAuthority = strings.Repeat("09", 32) }},
		{"malformed State signer", func(p *headlessRuntimePlan) { p.ClosedProfileAuthority = "not-a-key" }},
		{"missing reader response", func(p *headlessRuntimePlan) { p.ReaderPermission.ResponsePath = "" }},
		{"missing publisher request", func(p *headlessRuntimePlan) { p.PublisherPermission.RequestPath = "" }},
		{"missing Instance", func(p *headlessRuntimePlan) { p.ServiceInstanceRoot = "" }},
		{"shared request", func(p *headlessRuntimePlan) { p.PublisherPermission.RequestPath = p.ReaderPermission.RequestPath }},
		{"shared root", func(p *headlessRuntimePlan) { p.TextTokenRoot = p.NetworkStateRoot }},
		{"relative root", func(p *headlessRuntimePlan) { p.TextTokenRoot = "tokens" }},
		{"empty allocation", func(p *headlessRuntimePlan) { p.ReaderPermission.Maxima = [3]uint32{} }},
		{"reader over allocation", func(p *headlessRuntimePlan) { p.ReaderPermission.Maxima = [3]uint32{4096, 1, 0} }},
		{"publisher over allocation", func(p *headlessRuntimePlan) { p.PublisherPermission.Maxima = [3]uint32{16384, 0, 1} }},
		{"allocation overflow", func(p *headlessRuntimePlan) { p.ReaderPermission.Maxima = [3]uint32{^uint32(0), 2, 0} }},
		{"legacy profile", func(p *headlessRuntimePlan) { p.NetworkProfile = route.Profile }},
		{"legacy acquisition", func(p *headlessRuntimePlan) {
			p.TransitAcquisitionRoot = filepath.Join(filepath.Dir(p.TextTokenRoot), "transit")
		}},
		{"legacy byte allowance", func(p *headlessRuntimePlan) { p.BytesEachDirection = 4096 }},
		{"legacy corpus", func(p *headlessRuntimePlan) { p.AlphaCohort = "closed-alpha-1" }},
		{"unknown schema", func(p *headlessRuntimePlan) { p.Schema = "ardents-headless-runtime-v3" }},
		{"v1 with text settings", func(p *headlessRuntimePlan) {
			p.Schema = "ardents-headless-runtime-v1"
			p.NetworkProfile = route.Profile
			p.TransitAcquisitionRoot = filepath.Join(filepath.Dir(p.TextTokenRoot), "transit")
			p.BytesEachDirection = 4096
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := headlessTextPlanFixture(t)
			test.change(&plan)
			if _, err := loadHeadlessRuntimePlan(writeHeadlessTextPlan(t, plan)); err == nil {
				t.Fatal("invalid text plan accepted")
			}
		})
	}
}

func headlessTextPlanFixture(t *testing.T) headlessRuntimePlan {
	t.Helper()
	root := t.TempDir()
	path := func(name string) string { return filepath.Join(root, name) }
	return headlessRuntimePlan{
		Schema: "ardents-headless-runtime-v2", NetworkProfile: route.ClosedRouteProfile, ClosedProfileAuthority: strings.Repeat("02", 32),
		NetworkStateRoot: path("network"), EntryStateRoot: path("entry"), LocalRoleStateRoot: path("roles"), TextTokenRoot: path("tokens"),
		PublicationRoot: path("publication"), ServiceInstanceRoot: path("instance"),
		ApplicationSocket: path("application.sock"), AdministrationSocket: path("administration.sock"), TimeConfidenceFile: path("clock"),
		ReaderPermission:    headlessPermissionPlan{RequestPath: path("reader-request"), ResponsePath: path("reader-response"), Maxima: [3]uint32{1, 1, 1}},
		PublisherPermission: headlessPermissionPlan{RequestPath: path("publisher-request"), ResponsePath: path("publisher-response"), Maxima: [3]uint32{1, 1, 1}},
		NetworkID:           strings.Repeat("01", 32), NetworkAuthorities: []string{strings.Repeat("02", 32)}, NetworkThreshold: 1,
		BrokerID: strings.Repeat("03", 32), ConnectionPrincipal: strings.Repeat("04", 32), AdministrationPrincipal: strings.Repeat("05", 32),
	}
}

func writeHeadlessTextPlan(t *testing.T, plan headlessRuntimePlan) string {
	t.Helper()
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
