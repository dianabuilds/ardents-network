//go:build linux

package endpoint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTextWorkerInventoryRetainsFailedAndActivatingInstances(t *testing.T) {
	body := `[[["ardents-text-reader@0-12-997.service","","loaded","failed","failed","","/unit/a",0,"","/"],["ardents-text-reader@1-12-997.service","","loaded","activating","start","","/unit/b",0,"","/"]]]`
	answer := textManagerValue{Type: "a(ssssssouso)", Data: json.RawMessage(body)}
	found, err := decodeTextWorkerInstances(answer, "reader", "-12-997.service")
	if err != nil || len(found) != 2 || found["ardents-text-reader@0-12-997.service"].active != "failed" || found["ardents-text-reader@1-12-997.service"].active != "activating" {
		t.Fatalf("incomplete activation baseline: %v, %v", found, err)
	}
	for _, invalid := range []string{"null", "[null]", strings.Replace(body, "@1-12-997", "@1-13-997", 1), strings.Replace(body, "@1-12-997", "@0-12-997", 1), strings.Replace(body, `"activating"`, `null`, 1)} {
		answer.Data = json.RawMessage(invalid)
		if _, err := decodeTextWorkerInstances(answer, "reader", "-12-997.service"); err == nil {
			t.Fatal("ambiguous or incomplete system inventory admitted")
		}
	}
	answer.Data = json.RawMessage(`[[]]`)
	if found, err := decodeTextWorkerInstances(answer, "reader", "-12-997.service"); err != nil || found == nil || len(found) != 0 {
		t.Fatal("valid empty baseline refused")
	}
}

func TestTextWorkerInventoryNeverChoosesAmongConcurrentActivations(t *testing.T) {
	old := "ardents-text-reader@0-12-997.service"
	fresh := "ardents-text-reader@1-12-997.service"
	before := textWorkerListing{old: {active: "activating"}}
	// A previous activation becoming active is not a new job.
	if got, err := newTextWorkerInstance(before, textWorkerListing{old: {active: "active"}}, ""); err != nil || got != "" {
		t.Fatal("late old activation was selected")
	}
	if got, err := newTextWorkerInstance(before, textWorkerListing{old: {active: "active"}, fresh: {active: "activating"}}, ""); err != nil || got != fresh {
		t.Fatal("single fresh activation was lost")
	}
	for _, after := range []textWorkerListing{nil, {fresh: {active: "failed"}}, {fresh: {active: "inactive"}}, {fresh: {active: "deactivating"}}, {fresh: {active: "active"}, "ardents-text-reader@2-12-997.service": {active: "activating"}}} {
		if _, err := newTextWorkerInstance(before, after, ""); err == nil {
			t.Fatal("ambiguous or failed activation admitted")
		}
	}
}

func TestTextWorkerInvocationRejectsUnknownBytesAndBase64(t *testing.T) {
	valid := `[1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`
	for _, raw := range []string{`"AQAAAAAAAAAAAAAAAAAAAA=="`, `[1,null,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`, strings.Replace(valid, "[1,", "[256,", 1), strings.Replace(valid, "[1,", "[1.0,", 1), strings.Replace(valid, "[1,", "[-1,", 1), strings.Replace(valid, "[1,", "[0,", 1), `null`} {
		var id [16]byte
		if decodeTextWorkerInvocation(textManagerValue{Type: "ay", Data: json.RawMessage(raw)}, &id) {
			t.Fatal("unknown or malformed invocation admitted")
		}
	}
	var id [16]byte
	if !decodeTextWorkerInvocation(textManagerValue{Type: "ay", Data: json.RawMessage(valid)}, &id) || id != [16]byte{1} {
		t.Fatal("exact invocation refused")
	}
}

func TestTextWorkerInventoryWaitsForExactQueuedStart(t *testing.T) {
	const name = "ardents-text-reader@1-12-997.service"
	const pending = `[[["ardents-text-reader@1-12-997.service","","loaded","inactive","dead","","/unit/a",42,"start","/org/freedesktop/systemd1/job/42"]]]`
	for _, test := range []struct {
		name string
		body string
		want bool
	}{
		{"queued", pending, true},
		{"no-job", strings.Replace(pending, `42,"start","/org/freedesktop/systemd1/job/42"`, `0,"","/"`, 1), false},
		{"stop", strings.Replace(pending, `"start"`, `"stop"`, 1), false},
		{"foreign-job", strings.Replace(pending, `job/42`, `job/43`, 1), false},
		{"null-job", strings.Replace(pending, `,42,`, `,null,`, 1), false},
		{"unloaded", strings.Replace(pending, `"loaded"`, `"not-found"`, 1), false},
		{"unknown-substate", strings.Replace(pending, `"dead"`, `"unknown"`, 1), false},
		{"failed", strings.Replace(pending, `"inactive"`, `"failed"`, 1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			listing, err := decodeTextWorkerInstances(textManagerValue{Type: "a(ssssssouso)", Data: json.RawMessage(test.body)}, "reader", "-12-997.service")
			var got string
			if err == nil {
				got, err = newTextWorkerInstance(textWorkerListing{}, listing, "")
			}
			if test.want && (err != nil || got != name) {
				t.Fatalf("valid queued activation refused: %s, %v", got, err)
			}
			if !test.want && err == nil {
				t.Fatal("unverified queued activation selected")
			}
		})
	}
}

func TestTextWorkerInventoryRetainsQueuedCandidate(t *testing.T) {
	const old = "ardents-text-reader@0-12-997.service"
	const fresh = "ardents-text-reader@1-12-997.service"
	const other = "ardents-text-reader@2-12-997.service"
	before := textWorkerListing{old: {active: "activating"}}
	candidate := ""
	for _, state := range []textWorkerUnitState{{active: "inactive", pendingStart: true}, {active: "activating"}, {active: "active"}} {
		got, err := newTextWorkerInstance(before, textWorkerListing{old: {active: "active"}, fresh: state}, candidate)
		if err != nil || got != fresh {
			t.Fatalf("queued candidate transition refused: %v", err)
		}
		candidate = got
	}
	for _, after := range []textWorkerListing{
		{}, {other: {active: "active"}},
		{fresh: {active: "inactive", pendingStart: true}, other: {active: "activating"}},
		{old: {active: "active"}}, {fresh: {active: "inactive"}},
	} {
		if _, err := newTextWorkerInstance(before, after, candidate); err == nil {
			t.Fatal("candidate disappearance, substitution or ambiguity admitted")
		}
	}
}
