package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDescriptorReleasePlanRefusesForeignPurposeBeforeKeys(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	p := descriptorReleasePlan{Schema: "ardents-descriptor-initial-release-plan-v1", Network: "one-network", Reference: now, Expires: now.Add(time.Hour), Program: "/unopened/program", Descriptor: "/unopened/descriptor", Template: "/unopened/template", ProgramCustom: json.RawMessage(`{}`), GenerationCustom: json.RawMessage(`{}`)}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeDescriptorReleasePurpose(raw, now, descriptorReleaseInitial); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"ardents-source-initial-release-plan-v1", "ardents-source-forward-release-plan-v1", "ardents-forwarding-initial-release-plan-v1", "ardents-forwarding-forward-release-plan-v1", "ardents-descriptor-forward-release-plan-v1", "ardents-join-initial-release-plan-v1", "ardents-qualification-release-signing-plan-v1"} {
		if _, err := decodeDescriptorReleasePurpose([]byte(strings.Replace(string(raw), p.Schema, schema, 1)), now, descriptorReleaseInitial); err == nil {
			t.Fatal("foreign signing purpose accepted", schema)
		}
	}
	for _, bad := range []string{`{"network":"other",` + string(raw[1:]), `{"message":"arbitrary",` + string(raw[1:]), strings.Replace(string(raw), `"network":"one-network"`, `"network":null`, 1), string(raw) + ` {}`} {
		if _, err := decodeDescriptorReleasePurpose([]byte(bad), now, descriptorReleaseInitial); err == nil {
			t.Fatal("ambiguous public declaration accepted")
		}
	}
}
