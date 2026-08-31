package enrollment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestReadClosedAlphaInputMakesTheExactVerificationRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enrollment.json")
	raw, err := json.Marshal(ClosedAlphaInput{Schema: closedAlphaInputSchema, BundleRoot: "/bundle", Cohort: "cohort-1", Release: "alpha-1",
		Platform: "linux-amd64", ManifestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Environment: "alpha", Network: "network-1", TargetPath: "ardents/linux-amd64/endpoint"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := ReadClosedAlphaInput(path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC)
	request := input.Request("/bundle/ardents", at)
	if request.BundleRoot != input.BundleRoot || request.ExecutablePath != "/bundle/ardents" || request.Pin.Platform != input.Platform ||
		request.Architecture != runtime.GOARCH || !request.ReferenceTime.Equal(at) {
		t.Fatalf("enrollment request = %+v", request)
	}
}

func TestReadClosedAlphaInputRejectsUnknownAndIncompleteFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enrollment.json")
	if err := os.WriteFile(path, []byte(`{"schema":"ardents-alpha-enrollment-input-v1","unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClosedAlphaInput(path); err == nil {
		t.Fatal("unknown incomplete enrollment input was accepted")
	}
}
