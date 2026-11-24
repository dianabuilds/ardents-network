package main

import "github.com/dianabuilds/ardents-network/internal/admission/quota"

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	"strconv"
	"strings"
	"testing"
	"time"

	"runtime"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

type admissionBindingInput struct {
	Network   string              `json:"network"`
	Issuer    string              `json:"issuer"`
	Authority string              `json:"authority"`
	Profile   string              `json:"profile"`
	Duty      string              `json:"duty"`
	Start     string              `json:"start"`
	End       string              `json:"end"`
	Keys      []admissionKeyInput `json:"keys"`
}
type admissionKeyInput struct {
	Window string `json:"window"`
	Class  string `json:"class"`
	SPKI   string `json:"spki"`
}

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
	_, _, b := nodefixture.AdmissionCommandFixture(t, 2, 1, 2, 1)
	raw, err := json.Marshal(map[string]any{"root": t.TempDir(), "binding": admissionBindingConfig(b)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeAdmissionPlan(raw); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(string) string{
		func(s string) string { return strings.Replace(s, `"network":`, `"Network":`, 1) },
		func(s string) string { return strings.Replace(s, `"class":`, `"Class":`, 1) },
		func(s string) string { return strings.Replace(s, `"duty":"7"`, `"duty":"07"`, 1) },
		func(s string) string { return strings.Replace(s, `"duty":"7"`, `"duty":"7","duty":"7"`, 1) },
		func(s string) string { return strings.Replace(s, `"keys":[`, `"keys":null,"extra":[`, 1) },
		func(s string) string { return s + `{}` },
	} {
		if _, err := decodeAdmissionPlan([]byte(change(string(raw)))); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	if _, err := decodeAdmissionPlan([]byte(`{"root":"x","binding":null}`)); err == nil {
		t.Fatal("null accepted")
	}
	// Repeated equal IDs must not silently omit assignments while decoding.
	input := admissionBindingConfig(b)
	input.Issuer = input.Network
	raw, _ = json.Marshal(map[string]any{"root": t.TempDir(), "binding": input})
	p, err := decodeAdmissionPlan(raw)
	if err != nil || p.Binding.Network != p.Binding.Issuer || p.Binding.Network == [32]byte{} {
		t.Fatal("equal values lost field", err)
	}
}
func TestAdmissionUnsupportedOrCanceledBeforeFiles(t *testing.T) {
	var out, log bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	code := run(ctx, []string{"admission", "initialize", "--config", "nonexistent"}, &out, &log)
	if runtime.GOOS == "linux" && quota.Supported() {
		if code != 130 || !bytes.Contains(out.Bytes(), []byte("canceled")) {
			t.Fatal(code, &out)
		}
	} else {
		if code != 1 || !bytes.Contains(out.Bytes(), []byte("unsupported-platform")) {
			t.Fatal(code, &out)
		}
	}
}
