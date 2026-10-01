package runtimeplan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSourceDeclarationsDecodeWithoutOpeningCredentials(t *testing.T) {
	plan := sourceDeclarations(t)
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSource(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.NetworkID[0] != 0x11 || decoded.OrderSeed[0] != 0x22 ||
		decoded.Identities[0][0] != 0x33 || decoded.Identities[1][0] != 0x44 ||
		decoded.LeafKeyDigests[0][0] != 0x55 || len(decoded.Authorities) != 1 ||
		!decoded.ClockObservation.Equal(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("public Source projection differs from declarations: %+v", decoded)
	}
	for _, path := range []string{plan.LocalRoleStateRoot, plan.ClientCertificate, plan.ClientKey, plan.Sources[0].RootCA} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("decoder created or required %s: %v", path, err)
		}
	}
}

func TestSourceDeclarationsRefuseMalformedInputsBeforeCredentials(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Source)
	}{
		{"retired-schema", func(p *Source) { p.Schema = "ardents-test-source-plan-v1" }},
		{"missing-role-root", func(p *Source) { p.LocalRoleStateRoot = "" }},
		{"one-source", func(p *Source) { p.Sources = p.Sources[:1] }},
		{"short-network", func(p *Source) { p.NetworkID = "11" }},
		{"missing-authority", func(p *Source) { p.AuthorityPublic = nil }},
		{"too-many-authorities", func(p *Source) { p.AuthorityPublic = make([]string, 17) }},
		{"bad-clock", func(p *Source) { p.ClockObservedAt = "yesterday" }},
		{"short-order-seed", func(p *Source) { p.OrderSeed = "22" }},
		{"short-source-identity", func(p *Source) { p.Sources[1].Identity = "44" }},
		{"short-leaf-digest", func(p *Source) { p.Sources[0].LeafKeyDigest = "55" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := sourceDeclarations(t)
			test.change(&plan)
			raw, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeSource(raw); err == nil {
				t.Fatal("malformed Source declaration accepted")
			}
		})
	}
	plan := sourceDeclarations(t)
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string][]byte{
		"unknown-field": append([]byte(`{"extra":true,`), raw[1:]...),
		"trailing-json": append(append([]byte{}, raw...), []byte(` {}`)...),
		"oversize":      append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), 32<<10)...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeSource(input); err == nil {
				t.Fatal("malformed envelope accepted")
			}
		})
	}
}

func sourceDeclarations(t *testing.T) Source {
	t.Helper()
	root := t.TempDir()
	plan := Source{Schema: "ardents-source-plan-v1", NetworkID: strings.Repeat("11", 32),
		AuthorityPublic: []string{strings.Repeat("aa", 32)}, Threshold: 1,
		ClockObservedAt: "2026-09-30T12:00:00Z", OrderSeed: strings.Repeat("22", 32),
		LocalRoleStateRoot: filepath.Join(root, "role"), ClientCertificate: filepath.Join(root, "client.pem"),
		ClientKey: filepath.Join(root, "client.key")}
	for _, identity := range []string{"33", "44"} {
		plan.Sources = append(plan.Sources, SourceMember{Address: "127.0.0.1:12345", ServerName: "source.test",
			Identity: strings.Repeat(identity, 32), Family: identity, EndpointHandle: identity,
			RootCA: filepath.Join(root, "ca.pem"), LeafKeyDigest: strings.Repeat("55", 32)})
	}
	return plan
}
