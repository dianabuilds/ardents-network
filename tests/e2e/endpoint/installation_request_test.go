package endpoint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInstallationRequestRetainsIndependentInputValues(t *testing.T) {
	bundle := "/bundle/" + strings.Repeat("01", 32) + `/"release"`
	pin := strings.Repeat("ab", 32)
	history := `C:\test\floor`
	var decoded struct {
		Schema  string          `json:"schema"`
		Bundle  string          `json:"bundle_root"`
		Pin     string          `json:"manifest_sha256"`
		History string          `json:"release_floor_root"`
		Source  json.RawMessage `json:"source"`
	}
	if err := json.Unmarshal(InstallationRequest(t, bundle, pin, history), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Schema != "ardents-endpoint-installation-request-v1" || decoded.Bundle != bundle || decoded.Pin != pin || decoded.History != history || len(decoded.Source) == 0 {
		t.Fatalf("installation request fixture changed supplied values or omitted Source input: %+v", decoded)
	}
}
