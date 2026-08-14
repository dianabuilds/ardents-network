package recovery

import (
	"bytes"
	"testing"
)

func TestVerifyRejectsMalformedHostScope(t *testing.T) {
	valid := validEvidence(t).HostScope
	unknown := append(append([]byte(nil), valid[:len(valid)-1]...), []byte(`,"Unknown":1}`)...)
	for name, raw := range map[string][]byte{
		"unknown field":   unknown,
		"multiple values": append(append([]byte(nil), valid...), valid...),
		"noncanonical":    append([]byte(" "), valid...),
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
