package recovery

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestVerifyAcceptsHostScopeIndentedByTheEvidenceEnvelope(t *testing.T) {
	value := validEvidence(t)
	var indented bytes.Buffer
	if err := json.Indent(&indented, value.HostScope, "", "  "); err != nil {
		t.Fatal(err)
	}
	value.HostScope = indented.Bytes()
	if result := Verify(value); result.Verdict != "pass" {
		t.Fatalf("indented HostScope verdict = %+v, want pass", result)
	}
}

func TestVerifyRejectsMalformedHostScope(t *testing.T) {
	valid := validEvidence(t).HostScope
	unknown := append(append([]byte(nil), valid[:len(valid)-1]...), []byte(`,"Unknown":1}`)...)
	var fields map[string]any
	if err := json.Unmarshal(valid, &fields); err != nil {
		t.Fatal(err)
	}
	reordered, err := json.Marshal(fields)
	if err != nil || bytes.Equal(reordered, valid) {
		t.Fatalf("construct reordered HostScope: equal=%t err=%v", bytes.Equal(reordered, valid), err)
	}
	for name, raw := range map[string][]byte{
		"unknown field":   unknown,
		"multiple values": append(append([]byte(nil), valid...), valid...),
		"noncanonical":    reordered,
		"oversized":       bytes.Repeat([]byte{'{'}, 64<<10+1),
	} {
		t.Run(name, func(t *testing.T) {
			value := validEvidence(t)
			value.HostScope = raw
			if result := Verify(value); result.Verdict != "invalid" {
				t.Fatalf("malformed HostScope verdict = %+v, want invalid", result)
			}
		})
	}
}
