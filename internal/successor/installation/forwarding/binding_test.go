package forwarding

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func forwardingBindingVector() []byte {
	generation := strings.Repeat("a", 64)
	unit := forwardingUnitVector(generation)
	template := strings.Replace(unit, `"/opt/ardents-forwarding/generations/`+generation+`/ardents-node-linux-amd64"`, "@ARDENTS_FORWARDING_PROGRAM@", 1)
	template = strings.Replace(template, `"/opt/ardents-forwarding/binding.json"`, "@ARDENTS_FORWARDING_BINDING@", 1)
	template = strings.Replace(template, `"/var/lib/ardents-node-state" "/var/lib/ardents-node-participation" "/var/lib/ardents-node-spend" "/var/lib/ardents-node-hosting"`, "@ARDENTS_FORWARDING_WRITE_PATHS@", 1)
	return fmt.Appendf(nil, `{"schema":"ardents-forwarding-binding-v1","invocation":"forwarding","generation":"%s","program_target":"ardents/linux-amd64/node","generation_target":"ardents/linux-amd64/infrastructure/forwarding","program_sha256":"%s","template_sha256":"%x","inventory_sha256":"%s","targets_sha256":"%s","request_sha256":"%x","plan_sha256":"%s","unit_sha256":"%x","certificate_sha256":"%s","account":"ardents-node","uid":999,"gid":999,"history":"/var/lib/forwarding-release-history","request":%s}`+"\n", generation, strings.Repeat("b", 64), sha256.Sum256([]byte(template)), strings.Repeat("c", 64), strings.Repeat("d", 64), sha256.Sum256([]byte(forwardingRequestVector)), strings.Repeat("e", 64), sha256.Sum256([]byte(unit)), strings.Repeat("f", 64), strings.TrimSuffix(forwardingRequestVector, "\n"))
}

func TestForwardingBindingIndependentInitialBytesAndRefusals(t *testing.T) {
	raw := forwardingBindingVector()
	b, err := decodeBinding(raw)
	if err != nil || b.Account != "ardents-node" || b.UID != 999 || b.Request.HostingRoot != "/var/lib/ardents-node-hosting" {
		t.Fatal("independent complete binding refused", err)
	}
	for _, change := range []struct{ before, after string }{
		{`ardents-forwarding-binding-v1`, `ardents-source-binding-v1`},
		{`"invocation":"forwarding"`, `"invocation":"issuer"`},
		{`ardents/linux-amd64/infrastructure/forwarding`, `ardents/linux-amd64/infrastructure/source`},
		{`"account":"ardents-node"`, `"account":"ardents-source"`},
		{`"uid":999`, `"uid":0`}, {`"gid":999`, `"gid":998`},
		{`"uid":999`, `"uid":999,"uid":999`}, {`"uid":999`, `"extra":true,"uid":999`},
		{`"uid":999,"gid":999`, `"gid":999,"uid":999`},
		{`/var/lib/forwarding-release-history`, `/var/lib/ardents-node-hosting/history`},
		{`/var/lib/forwarding-release-history`, `/opt/ardents-forwarding.forwarding-initial-journal`},
		{`/var/lib/forwarding-release-history`, `/opt/ardents-forwarding.forwarding-initial-pending`},
		{`/var/lib/ardents-node-state`, `/var/lib/foreign-state`},
		{strings.Repeat("a", 64), strings.Repeat("A", 64)},
	} {
		if _, err := decodeBinding([]byte(strings.Replace(string(raw), change.before, change.after, 1))); err == nil {
			t.Fatal("foreign/ambiguous initial binding accepted", change)
		}
	}
	for _, other := range [][]byte{nil, raw[:len(raw)-1], append(append([]byte(nil), raw...), '\n')} {
		if _, err := decodeBinding(other); err == nil {
			t.Fatal("noncanonical binding accepted")
		}
	}
	for n := 0; n < 64; n++ {
		path := "/opt/ardents-forwarding.forwarding-first-start-journal"
		if n != 0 {
			path = fmt.Sprintf("/opt/ardents-forwarding.forwarding-invocation-%08d-journal", n)
		}
		for _, history := range []string{path, path + "/history"} {
			if _, err := decodeBinding([]byte(strings.Replace(string(raw), "/var/lib/forwarding-release-history", history, 1))); err == nil {
				t.Fatal("Release history borrowed future invocation journal", history)
			}
		}
		retirement := fmt.Sprintf("/opt/ardents-forwarding.forwarding-retirement-%08d-journal", n)
		for _, history := range []string{retirement, retirement + "/history"} {
			if _, err := decodeBinding([]byte(strings.Replace(string(raw), "/var/lib/forwarding-release-history", history, 1))); err == nil {
				t.Fatal("Release history borrowed a future retirement journal", history)
			}
		}
	}
}
