package main

import "github.com/dianabuilds/ardents-network/internal/successor/admission/quota"

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func admissionBindingConfig(b quota.LedgerBinding) admissionBindingInput {
	input := admissionBindingInput{Network: hex.EncodeToString(b.Network[:]), Issuer: hex.EncodeToString(b.Issuer[:]), Authority: hex.EncodeToString(b.Authority[:]), Profile: hex.EncodeToString(b.Profile[:]), Duty: strconv.FormatUint(b.Duty, 10), Start: b.Start.Format(time.RFC3339), End: b.End.Format(time.RFC3339)}
	for _, key := range b.Keys {
		input.Keys = append(input.Keys, admissionKeyInput{time.Unix(int64(key.Window), 0).UTC().Format(time.RFC3339), strconv.Itoa(int(key.Class)), hex.EncodeToString(key.SPKI)})
	}
	return input
}
func admissionFactsConfig(f admission.Facts) map[string]string {
	return map[string]string{"network": hex.EncodeToString(f.Network[:]), "issuer": hex.EncodeToString(f.Issuer[:]), "authority": hex.EncodeToString(f.Authority[:]), "holder": hex.EncodeToString(f.Holder[:]), "duty": strconv.FormatUint(f.Duty, 10), "duty_not_before": f.DutyNotBefore.Format(time.RFC3339), "duty_not_after": f.DutyNotAfter.Format(time.RFC3339), "now": f.Now.Format(time.RFC3339), "class": strconv.Itoa(int(f.Class)), "count": strconv.Itoa(int(f.Count))}
}
func TestAdmissionExactInput(t *testing.T) {
	_, f, b := admissionCommandFixture(t, 2, 1, 2, 1)
	raw, err := json.Marshal(map[string]any{"root": t.TempDir(), "binding": admissionBindingConfig(b)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeAdmissionPlan(raw, "initialize"); err != nil {
		t.Fatal(err)
	}
	debit, err := json.Marshal(map[string]any{"root": t.TempDir(), "binding": admissionBindingConfig(b), "batch_file": t.TempDir(), "facts": admissionFactsConfig(f), "kind": "bootstrap"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeAdmissionPlan(debit, "debit"); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		strings.Replace(string(debit), `"holder":`, `"Holder":`, 1),
		strings.Replace(string(debit), `"kind":"bootstrap"`, `"kind":"unknown"`, 1),
		strings.Replace(string(debit), `"count":"1"`, `"count":"01"`, 1),
	} {
		if _, err := decodeAdmissionPlan([]byte(invalid), "debit"); err == nil {
			t.Fatal("invalid debit input accepted")
		}
	}
	for _, change := range []func(string) string{
		func(s string) string { return strings.Replace(s, `"network":`, `"Network":`, 1) },
		func(s string) string { return strings.Replace(s, `"class":`, `"Class":`, 1) },
		func(s string) string { return strings.Replace(s, `"duty":"7"`, `"duty":"07"`, 1) },
		func(s string) string { return strings.Replace(s, `"duty":"7"`, `"duty":"7","duty":"7"`, 1) },
		func(s string) string { return strings.Replace(s, `"keys":[`, `"keys":null,"extra":[`, 1) },
		func(s string) string { return s + `{}` },
	} {
		if _, err := decodeAdmissionPlan([]byte(change(string(raw))), "initialize"); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	if _, err := decodeAdmissionPlan([]byte(`{"root":"x","binding":null}`), "initialize"); err == nil {
		t.Fatal("null accepted")
	}
	// Repeated equal IDs must not silently omit assignments while decoding.
	input := admissionBindingConfig(b)
	input.Issuer = input.Network
	raw, _ = json.Marshal(map[string]any{"root": t.TempDir(), "binding": input})
	p, err := decodeAdmissionPlan(raw, "initialize")
	if err != nil || p.Binding.Network != p.Binding.Issuer || p.Binding.Network == [32]byte{} {
		t.Fatal("equal values lost field", err)
	}
}
func TestAdmissionUnsupportedOrCanceledBeforeFiles(t *testing.T) {
	var out, log bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	code := run(ctx, []string{"admission", "initialize", "--config", "nonexistent"}, &out, &log)
	if quota.Supported() {
		if code != 130 || !bytes.Contains(out.Bytes(), []byte("canceled")) {
			t.Fatal(code, &out)
		}
	} else {
		if code != 1 || !bytes.Contains(out.Bytes(), []byte("unsupported-platform")) {
			t.Fatal(code, &out)
		}
	}
}
