package main

import "github.com/dianabuilds/ardents-network/internal/admission/quota"

import (
	"bytes"
	"context"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

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
func TestAdmissionUnsupportedOrCanceledBeforeFiles(t *testing.T) {
	var out, log bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	code := run(ctx, []string{"admission", "initialize", "--config", "nonexistent"}, &out, &log)
	if quota.OperatorAvailability() == "" {
		if code != 130 || !bytes.Contains(out.Bytes(), []byte("canceled")) {
			t.Fatal(code, &out)
		}
	} else {
		if code != 1 || !bytes.Contains(out.Bytes(), []byte("unsupported-platform")) {
			t.Fatal(code, &out)
		}
	}
}
