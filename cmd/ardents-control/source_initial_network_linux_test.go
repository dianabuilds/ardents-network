package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSourceInitialNetworkPlanKeepsDistinctPurposeAndPublicPins(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	p := sourceInitialNetworkPlan{Schema: "ardents-source-initial-network-plan-v1", Reference: now, Network: sourceInitialID{1}, Node: sourceInitialID{2}, NodeKey: sourceInitialID{3}, StateKey: sourceInitialID{4},
		Family: "one-effective-controller", Endpoint: "127.0.0.1:4100", From: now.Add(-time.Minute), Until: now.Add(time.Hour), Seed: sourceInitialID{5}, Domains: []string{"domain-a"}}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeSourceInitialNetwork(raw, now); err != nil {
		t.Fatal("bounded honest declaration", err)
	}
	for name, edit := range map[string]func(*sourceInitialNetworkPlan){
		"same-purpose-key": func(p *sourceInitialNetworkPlan) { p.StateKey = p.NodeKey },
		"missing-network":  func(p *sourceInitialNetworkPlan) { p.Network = sourceInitialID{} },
		"future-record":    func(p *sourceInitialNetworkPlan) { p.From = now.Add(time.Minute) },
		"expired-record":   func(p *sourceInitialNetworkPlan) { p.Until = now.Add(-time.Second) },
		"stale-review":     func(p *sourceInitialNetworkPlan) { p.Reference = now.Add(-6 * time.Minute) },
		"hostname":         func(p *sourceInitialNetworkPlan) { p.Endpoint = "source.test:4100" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := p
			edit(&changed)
			raw, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeSourceInitialNetwork(raw, now); err == nil {
				t.Fatal("wrong initial signing declaration admitted")
			}
		})
	}
	for _, bad := range [][]byte{append([]byte(`{"family":"shadow",`), raw[1:]...), append([]byte(`{"raw_message":"arbitrary",`), raw[1:]...), []byte(strings.Replace(string(raw), "01"+strings.Repeat("0", 62), "01", 1)), append(bytes.Clone(raw), []byte(" {}")...)} {
		if _, err := decodeSourceInitialNetwork(bad, now); err == nil {
			t.Fatal("ambiguous/truncated public declaration admitted")
		}
	}
}
