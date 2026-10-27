package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestForwardingReleasePlanRefusesForeignPurposeBeforeKeys(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	p := forwardingReleasePlan{Schema: "ardents-forwarding-initial-release-plan-v1", Network: "one-network", Reference: now, Expires: now.Add(time.Hour), Program: "/unopened/program", Descriptor: "/unopened/descriptor", Template: "/unopened/template", ProgramCustom: json.RawMessage(`{}`), GenerationCustom: json.RawMessage(`{}`)}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeForwardingReleasePlan(raw, now); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"ardents-source-initial-release-plan-v1", "ardents-source-forward-release-plan-v1", "ardents-forwarding-forward-release-plan-v1", "ardents-qualification-release-signing-plan-v1"} {
		if _, err := decodeForwardingReleasePlan([]byte(strings.Replace(string(raw), p.Schema, schema, 1)), now); err == nil {
			t.Fatal("foreign signing purpose accepted", schema)
		}
	}
	for _, bad := range []string{`{"network":"other",` + string(raw[1:]), `{"message":"arbitrary",` + string(raw[1:]), strings.Replace(string(raw), `"network":"one-network"`, `"network":null`, 1), string(raw) + ` {}`} {
		if _, err := decodeForwardingReleasePlan([]byte(bad), now); err == nil {
			t.Fatal("ambiguous public declaration accepted")
		}
	}
}

func TestForwardingForwardReleasePlanKeepsFixedPurposeAndBounds(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	p := forwardingReleasePlan{Schema: "ardents-forwarding-forward-release-plan-v1", Network: "one-network", Reference: now, Expires: now.Add(time.Hour), Program: "/unopened/program", Descriptor: "/unopened/descriptor", Template: "/unopened/template", ProgramCustom: json.RawMessage(`{}`), GenerationCustom: json.RawMessage(`{}`)}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeForwardingReleasePurpose(raw, now, forwardingReleaseForward); err != nil {
		t.Fatal("separate forward declaration grammar", err)
	}
	if _, err := decodeForwardingReleasePlan(raw, now); err == nil {
		t.Fatal("forward declaration became initial permission")
	}
	for _, schema := range []string{"ardents-forwarding-initial-release-plan-v1", "ardents-source-forward-release-plan-v1", "ardents-forwarding-timestamp-release-plan-v1"} {
		if _, err := decodeForwardingReleasePurpose([]byte(strings.Replace(string(raw), p.Schema, schema, 1)), now, forwardingReleaseForward); err == nil {
			t.Fatal("foreign signing purpose", schema)
		}
	}
	if _, err := decodeForwardingReleasePurpose(raw, now, forwardingReleasePurpose(255)); err == nil {
		t.Fatal("unknown signing purpose accepted")
	}
	for _, bad := range []string{`{"network":"foreign",` + string(raw[1:]), `{"target":"arbitrary",` + string(raw[1:]), string(raw) + ` {}`} {
		if _, err := decodeForwardingReleasePurpose([]byte(bad), now, forwardingReleaseForward); err == nil {
			t.Fatal("ambiguous forward declaration")
		}
	}
	if _, err := decodeForwardingReleasePurpose(raw, now.Add(2*time.Hour), forwardingReleaseForward); err == nil {
		t.Fatal("expired forward declaration")
	}
}
