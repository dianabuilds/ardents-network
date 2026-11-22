//go:build linux

package worker

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
			properties := properties{}
			if test.signature != "" {
				properties["Guard"] = value{Type: test.signature, Data: json.RawMessage(test.body)}
			}
			if properties.exact("Guard", "b", false) {
				t.Fatal("unknown hardening became an accepting false observation")
			}
		})
	}
	properties := properties{"Guard": {Type: "b", Data: json.RawMessage(" false ")}}
	if !properties.exact("Guard", "b", false) {
		t.Fatal("valid typed observation refused")
	}
}

func TestTextManagerResponseLimitCannotBeBypassedByReaderFrom(t *testing.T) {
	output := &managerOutput{}
	_, err := io.Copy(output, io.LimitReader(strings.NewReader(strings.Repeat("x", 600<<10)), 600<<10))
	if err == nil || len(output.Bytes()) > 512<<10 {
		t.Fatal("system manager output bypassed the finite buffer")
	}
}

func TestDocumentWorkerUnitRejectsForeignAndAmbiguousInstances(t *testing.T) {
	for _, name := range []string{"ssh.service", "ardents-text-publisher@.service", "ardents-text-publisher@1-0-997.service",
		"ardents-stream-qualification-publisher@0-12-997.service",
		"ardents-text-publisher@1-12-0.service", "ardents-text-publisher@01-12-997.service", "ardents-text-publisher@1-12-997.service/extra",
		"ardents-text-reader@1-12-997.service", "ardents-text-publisher@1-12-997.service\n"} {
		if validUnit(textInventory, name, "publisher") {
			t.Fatalf("admitted foreign unit %q", name)
		}
	}
	if !validUnit(textInventory, "ardents-text-publisher@0-12-997.service", "publisher") {
		t.Fatal("first exact socket instance refused")
	}
	if !validUnit(streamInventory, "ardents-stream-qualification-publisher@0-12-997.service", "publisher") {
		t.Fatal("first exact qualification socket instance refused")
	}
	if validUnit(streamInventory, "ardents-text-publisher@0-12-997.service", "publisher") {
		t.Fatal("stream inventory admitted an ordinary text unit")
	}
}

func TestDocumentWorkerExecutableObservationRejectsNullAndExtraCommands(t *testing.T) {
	valid := `[["/ardents-text",["/ardents-text","worker-publisher"],[],1,1,0,0,42,0,0]]`
	for _, raw := range []string{
		strings.Replace(valid, "[],1,1", "null,1,1", 1), strings.Replace(valid, "[],1,1", `["privileged"],1,1`, 1),
		strings.Replace(valid, "worker-publisher", "worker-reader", 1), strings.Replace(valid, ",42,", ",43,", 1),
		strings.Replace(valid, ",1,1,", ",1,0,", 1), strings.Replace(valid, `["/ardents-text","worker-publisher"]`, `["/ardents-text","worker-publisher","extra"]`, 1),
		"[" + valid[1:len(valid)-1] + "," + valid[1:len(valid)-1] + "]",
	} {
		properties := properties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: json.RawMessage(raw)}}
		if verifyExec(textInventory, properties, "publisher", 42) == nil {
			t.Fatal("unverified executable observation admitted")
		}
	}
	properties := properties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: json.RawMessage(valid)}}
	if err := verifyExec(textInventory, properties, "publisher", 42); err != nil {
		t.Fatal(err)
	}
}

func TestStreamWorkerExecutableObservationBindsQualificationProgram(t *testing.T) {
	valid := `[["/ardents-stream-qualification",["/ardents-stream-qualification","worker-publisher"],[],1,1,0,0,42,0,0]]`
	observed := properties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: json.RawMessage(valid)}}
	if err := verifyExec(streamInventory, observed, "publisher", 42); err != nil {
		t.Fatal(err)
	}
	if verifyExec(textInventory, observed, "publisher", 42) == nil {
		t.Fatal("text inventory admitted the qualification executable")
	}
	text := properties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: json.RawMessage(`[["/ardents-text",["/ardents-text","worker-publisher"],[],1,1,0,0,42,0,0]]`)}}
	if verifyExec(streamInventory, text, "publisher", 42) == nil {
		t.Fatal("stream inventory admitted the ordinary text executable")
	}
}
