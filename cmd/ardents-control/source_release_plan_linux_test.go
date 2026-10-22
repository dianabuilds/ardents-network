package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestSourceReleasePlanKeepsDistinctClosedCurrentPurpose(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	plan := sourceReleasePlan{Schema: "ardents-source-initial-release-plan-v1", Network: "one-network", Reference: now, Expires: now.Add(time.Hour),
		Program: "/unopened/program", Descriptor: "/unopened/descriptor", Template: "/unopened/template", ProgramCustom: json.RawMessage(`{}`), GenerationCustom: json.RawMessage(`{}`)}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeSourceReleasePlan(raw, now); err != nil {
		t.Fatal("complete current declaration", err)
	}
	for name, bad := range map[string][]byte{
		"endpoint-purpose": bytes.ReplaceAll(raw, []byte("ardents-source-initial-release-plan-v1"), []byte("ardents-qualification-release-signing-plan-v1")),
		"duplicate":        append([]byte(`{"network":"shadow",`), raw[1:]...),
		"unknown-message":  append([]byte(`{"message":"arbitrary signing input",`), raw[1:]...),
		"null":             bytes.Replace(raw, []byte(`"program":"/unopened/program"`), []byte(`"program":null`), 1),
		"relative":         bytes.Replace(raw, []byte("/unopened/program"), []byte("relative"), 1),
		"trailing":         append(bytes.Clone(raw), []byte(` {}`)...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeSourceReleasePlan(bad, now); err == nil {
				t.Fatal("foreign/ambiguous purpose admitted")
			}
		})
	}
	for name, at := range map[string]time.Time{"stale": now.Add(6 * time.Minute), "future-reference": now.Add(-2 * time.Minute), "expired": now.Add(2 * time.Hour)} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeSourceReleasePlan(raw, at); err == nil {
				t.Fatal("uncurrent signing time admitted")
			}
		})
	}
	plan.Expires = now.Add(8 * 24 * time.Hour)
	raw, _ = json.Marshal(plan)
	if _, err := decodeSourceReleasePlan(raw, now); err == nil {
		t.Fatal("unbounded metadata lifetime admitted")
	}
}

func TestSourceReleasePrivateKeyParserRetainsCallerInput(t *testing.T) {
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x63}, ed25519.SeedSize))
	defer clear(key)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(der)
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	defer clear(raw)
	before := bytes.Clone(raw)
	defer clear(before)
	parsed, err := parseSourceSigningKey(raw)
	if err != nil || !bytes.Equal(parsed, key) || !bytes.Equal(raw, before) {
		t.Fatal("original key ownership lost", err)
	}
	clear(parsed)
	for _, bad := range [][]byte{append(bytes.Clone(raw), '\n'), []byte(strings.Replace(string(raw), "PRIVATE KEY", "PUBLIC KEY", 1)), []byte("arbitrary private material")} {
		if parsed, err := parseSourceSigningKey(bad); err == nil {
			clear(parsed)
			t.Fatal("mixed/foreign key purpose admitted")
		}
		clear(bad)
	}
}
