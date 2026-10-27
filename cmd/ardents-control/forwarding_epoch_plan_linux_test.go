package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func forwardingEpochPlanFixture(t *testing.T, now time.Time) forwardingEpochPlan {
	t.Helper()
	state, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clear(key)
	p := forwardingEpochPlan{Schema: "ardents-forwarding-epoch-plan-v1", Reference: now, Network: strings.Repeat("01", 32), Authority: hex.EncodeToString(state), Seed: strings.Repeat("04", 32), Node: strings.Repeat("03", 32), From: now.Add(-time.Minute), Until: now.Add(time.Hour), Domains: []string{"a", "b"}}
	for _, id := range []string{strings.Repeat("03", 32), strings.Repeat("02", 32)} {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		port := "9402"
		if id == p.Node {
			port = "9403"
		}
		declaration := forwardingRecordPlan{Schema: "ardents-forwarding-node-record-plan-v1", Reference: now.Add(-6 * time.Minute), Network: p.Network, Node: id, PublicKey: hex.EncodeToString(pub), Family: "one-controlled-family", Endpoint: "127.0.0.1:" + port, From: now.Add(-10 * time.Minute), Until: now.Add(2 * time.Hour), Capacity: 1}
		message, err := declaration.prepare(declaration.Reference)
		if err != nil {
			clear(key)
			t.Fatal(err)
		}
		signed := append(bytes.Clone(message), ed25519.Sign(key, message)...)
		clear(key)
		p.Records = append(p.Records, forwardingEpochRecord{declaration, signed})
	}
	return p
}

func TestForwardingEpochPreparationPreservesOrderedSignedRecordsAndSeed(t *testing.T) {
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	p := forwardingEpochPlanFixture(t, now)
	message, err := p.prepare(now)
	if err != nil {
		t.Fatal("valid historical signed Records refused", err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	_, again, err := decodeForwardingEpochPlan(raw, now)
	if err != nil || !bytes.Equal(message, again) {
		t.Fatal("public Epoch plan changed bytes", err)
	}
	for _, mutate := range []func(*forwardingEpochPlan){
		func(p *forwardingEpochPlan) { p.Schema = "ardents-source-initial-network-plan-v1" },
		func(p *forwardingEpochPlan) { p.Authority = p.Records[0].Declaration.PublicKey },
		func(p *forwardingEpochPlan) { p.Seed = strings.Repeat("00", 32) },
		func(p *forwardingEpochPlan) { p.Node = strings.Repeat("05", 32) },
		func(p *forwardingEpochPlan) { p.Domains = []string{"b", "a"} },
		func(p *forwardingEpochPlan) { p.Domains = []string{"a", "a"} },
		func(p *forwardingEpochPlan) { p.Reference = now.Add(-6 * time.Minute) },
		func(p *forwardingEpochPlan) { p.Reference = now.Add(2 * time.Minute) },
		func(p *forwardingEpochPlan) { p.Until = now.Add(3 * time.Hour) },
		func(p *forwardingEpochPlan) { p.Records = p.Records[:0] },
		func(p *forwardingEpochPlan) { p.Records[0].Signed[len(p.Records[0].Signed)-1] ^= 1 },
		func(p *forwardingEpochPlan) { p.Records[0].Declaration.Node = p.Records[1].Declaration.Node },
		func(p *forwardingEpochPlan) { p.Records = append(p.Records, p.Records[0]) },
	} {
		var changed forwardingEpochPlan
		if json.Unmarshal(raw, &changed) != nil {
			t.Fatal("fixture unreadable")
		}
		mutate(&changed)
		if _, err := changed.prepare(now); err == nil {
			t.Fatal("changed/ambiguous Epoch declaration accepted")
		}
	}
	for _, args := range [][]string{{"--record", "a", "--record", "b"}, {"--domain=a", "--domain=b"}, {"--record-plan", "a", "--record-plan", "b"}} {
		if !forwardingEpochUniqueOptions(args) {
			t.Fatal("ordered repeated input refused")
		}
	}
	if forwardingEpochUniqueOptions([]string{"--assignment-seed", "a", "--assignment-seed=b"}) {
		t.Fatal("duplicate seed permits implicit redraw")
	}
}
