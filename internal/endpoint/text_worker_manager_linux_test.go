//go:build linux

package endpoint

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestTextManagerPropertiesKeepAbsenceAndTypeDistinctFromZero(t *testing.T) {
	for _, test := range []struct{ name, signature, body string }{
		{"missing", "", ""}, {"null", "b", "null"}, {"wrong type", "s", "false"}, {"string instead of bool", "b", `"false"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			properties := textManagerProperties{}
			if test.signature != "" {
				properties["Guard"] = textManagerValue{Type: test.signature, Data: json.RawMessage(test.body)}
			}
			if properties.exact("Guard", "b", false) {
				t.Fatal("unknown hardening became an accepting false observation")
			}
		})
	}
	properties := textManagerProperties{"Guard": {Type: "b", Data: json.RawMessage(" false ")}}
	if !properties.exact("Guard", "b", false) {
		t.Fatal("valid typed observation refused")
	}
}

func TestTextManagerResponseLimitCannotBeBypassedByReaderFrom(t *testing.T) {
	output := &textManagerOutput{}
	_, err := io.Copy(output, io.LimitReader(strings.NewReader(strings.Repeat("x", 600<<10)), 600<<10))
	if err == nil || len(output.Bytes()) > 512<<10 {
		t.Fatal("system manager output bypassed the finite buffer")
	}
}

func TestTextWorkerUnitRejectsForeignAndAmbiguousInstances(t *testing.T) {
	for _, name := range []string{"ssh.service", "ardents-text-publisher@.service", "ardents-text-publisher@1-0-997.service",
		"ardents-text-publisher@1-12-0.service", "ardents-text-publisher@01-12-997.service", "ardents-text-publisher@1-12-997.service/extra",
		"ardents-text-reader@1-12-997.service", "ardents-text-publisher@1-12-997.service\n"} {
		if textWorkerUnit(name, "publisher") {
			t.Fatalf("admitted foreign unit %q", name)
		}
	}
	if !textWorkerUnit("ardents-text-publisher@0-12-997.service", "publisher") {
		t.Fatal("first exact socket instance refused")
	}
}

func TestTextWorkerExecutableObservationRejectsNullAndExtraCommands(t *testing.T) {
	valid := `[["/ardents-text",["/ardents-text","worker-publisher"],[],1,1,0,0,42,0,0]]`
	for _, raw := range []string{
		strings.Replace(valid, "[],1,1", "null,1,1", 1), strings.Replace(valid, "[],1,1", `["privileged"],1,1`, 1),
		strings.Replace(valid, "worker-publisher", "worker-reader", 1), strings.Replace(valid, ",42,", ",43,", 1),
		strings.Replace(valid, ",1,1,", ",1,0,", 1), strings.Replace(valid, `["/ardents-text","worker-publisher"]`, `["/ardents-text","worker-publisher","extra"]`, 1),
		"[" + valid[1:len(valid)-1] + "," + valid[1:len(valid)-1] + "]",
	} {
		properties := textManagerProperties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: json.RawMessage(raw)}}
		if verifyTextWorkerExec(properties, "publisher", 42) == nil {
			t.Fatal("unverified executable observation admitted")
		}
	}
	properties := textManagerProperties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: json.RawMessage(valid)}}
	if err := verifyTextWorkerExec(properties, "publisher", 42); err != nil {
		t.Fatal(err)
	}
}
