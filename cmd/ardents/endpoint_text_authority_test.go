package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestHeadlessTextSourceConfigurationRetainsExplicitSigner(t *testing.T) {
	plan := headlessTextPlanFixture(t)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), DNSNames: []string{"source.test"}}
	raw, err := x509.CreateCertificate(rand.Reader, certificate, certificate, public, private)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	certPath := write("source.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}))
	keyPath := write("source-key.pem", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}))
	source := sourcePlan{Schema: "ardents-source-plan-v1", NetworkID: plan.NetworkID, AuthorityPublic: plan.NetworkAuthorities, Threshold: plan.NetworkThreshold,
		ClockObservedAt: now.UTC().Format(time.RFC3339), ClockObservationFile: plan.TimeConfidenceFile, OrderSeed: strings.Repeat("07", 32), RefreshIntervalMS: 1000,
		LocalRoleStateRoot: plan.LocalRoleStateRoot, ClientCertificate: certPath, ClientKey: keyPath}
	for _, value := range []string{"08", "09"} {
		source.Sources = append(source.Sources, sourcePlanMember{Address: "127.0.0.1:12345", ServerName: "source.test", Identity: strings.Repeat(value, 32), Family: value, EndpointHandle: value, RootCA: certPath, LeafKeyDigest: strings.Repeat(value, 32)})
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	plan.NetworkSourcePlan = write("sources.json", encoded)
	decoded, err := loadHeadlessRuntimePlan(writeHeadlessTextPlan(t, plan))
	if err != nil {
		t.Fatal(err)
	}
	config, refresh, err := headlessNetworkConfig(decoded, time.Now)
	if err != nil || !refresh || config.AcceptedProfile != route.ClosedRouteProfile || hex.EncodeToString(config.ClosedProfileAuthority) != plan.ClosedProfileAuthority {
		t.Fatalf("Source-backed runtime lost selected State signer: refresh=%v, %v", refresh, err)
	}
}

func TestHeadlessLegacyPlanRejectsClosedSignerAlone(t *testing.T) {
	plan := headlessTextPlanFixture(t)
	plan.Schema, plan.NetworkProfile = "ardents-headless-runtime-v1", route.Profile
	plan.TextTokenRoot, plan.ClosedProfileAuthority = "", ""
	plan.ReaderPermission, plan.PublisherPermission = headlessPermissionPlan{}, headlessPermissionPlan{}
	plan.TransitAcquisitionRoot, plan.BytesEachDirection = filepath.Join(t.TempDir(), "transit"), 4096
	if _, err := loadHeadlessRuntimePlan(writeHeadlessTextPlan(t, plan)); err != nil {
		t.Fatalf("valid legacy baseline: %v", err)
	}
	plan.ClosedProfileAuthority = plan.NetworkAuthorities[0]
	if _, err := loadHeadlessRuntimePlan(writeHeadlessTextPlan(t, plan)); err == nil || !strings.Contains(err.Error(), "closed profile authority requires text runtime plan v2") {
		t.Fatalf("legacy plan accepted closed signer or failed elsewhere: %v", err)
	}
}
