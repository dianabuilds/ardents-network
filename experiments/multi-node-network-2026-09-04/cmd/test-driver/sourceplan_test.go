//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWritePlansBindClientCertificateToDeclaredClientRoot(t *testing.T) {
	fixturesDir := writeSourcePlanFixture(t)
	clientPEM, err := os.ReadFile(filepath.Join(fixturesDir, "client.pem"))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(clientPEM)
	if block == nil {
		t.Fatal("client certificate PEM is empty")
	}
	client, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"source-a.json", "source-b.json"} {
		raw, err := os.ReadFile(filepath.Join(fixturesDir, name))
		if err != nil {
			t.Fatal(err)
		}
		var plan SourceServerPlan
		if err := json.Unmarshal(raw, &plan); err != nil {
			t.Fatal(err)
		}
		rootPEM, err := os.ReadFile(plan.ClientRoot)
		if err != nil {
			t.Fatal(err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(rootPEM) {
			t.Fatalf("%s client root is not a certificate", name)
		}
		if _, err := client.Verify(x509.VerifyOptions{
			Roots: roots, CurrentTime: sourcePlanTestTime,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}); err != nil {
			t.Fatalf("%s does not trust the generated client certificate: %v", name, err)
		}
	}
}

func TestWritePlansDeclareLiveClockObservationFile(t *testing.T) {
	fixturesDir := writeSourcePlanFixture(t)
	raw, err := os.ReadFile(filepath.Join(fixturesDir, "client.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		ClockObservationFile string `json:"clock_observation_file"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(fixturesDir, "clock.observation")
	if plan.ClockObservationFile != want {
		t.Fatalf("clock observation file = %q, want %q", plan.ClockObservationFile, want)
	}
}

var sourcePlanTestTime = time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)

func writeSourcePlanFixture(t *testing.T) string {
	t.Helper()
	fixturesDir := t.TempDir()
	authorityPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	fixtures := Fixtures{
		NetworkIDHex:    strings.Repeat("11", 32),
		AuthorityPublic: authorityPrivate.Public().(ed25519.PublicKey),
	}
	clientPin, sourceAPin, sourceBPin, err := WriteCerts(
		filepath.Join(fixturesDir, "source-ca.pem"),
		filepath.Join(fixturesDir, "source-a.pem"), filepath.Join(fixturesDir, "source-a-key.pem"),
		filepath.Join(fixturesDir, "source-b.pem"), filepath.Join(fixturesDir, "source-b-key.pem"),
		filepath.Join(fixturesDir, "client-ca.pem"),
		filepath.Join(fixturesDir, "client.pem"), filepath.Join(fixturesDir, "client-key.pem"),
		sourcePlanTestTime,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePlans(fixturesDir, filepath.Join(fixturesDir, "source-a-state"),
		filepath.Join(fixturesDir, "source-b-state"), DefaultSourceAddressA, DefaultSourceAddressB,
		fixtures, clientPin, sourceAPin, sourceBPin, sourcePlanTestTime); err != nil {
		t.Fatal(err)
	}
	return fixturesDir
}
