package qualification_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFiniteSourceManifestIsFrozen(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Schema                 string    `json:"schema"`
		SourceCount            int       `json:"source_count"`
		RequestMagic           string    `json:"request_magic"`
		ResponseMagic          string    `json:"response_magic"`
		LatestOpcode           byte      `json:"latest_opcode"`
		ByDigestOpcode         byte      `json:"by_digest_opcode"`
		MaximumBundleBytes     int       `json:"maximum_bundle_bytes"`
		MaterializationIndex   uint32    `json:"materialization_index"`
		ExpectedSourceAttempts uint16    `json:"expected_source_attempts"`
		ExpectedEventKind      string    `json:"expected_event_kind"`
		OrderSeed              string    `json:"order_seed"`
		ConnectTimeout         int       `json:"connect_timeout_ms"`
		TLSTimeout             int       `json:"tls_timeout_ms"`
		HeaderTimeout          int       `json:"request_header_timeout_ms"`
		TotalTimeout           int       `json:"request_total_timeout_ms"`
		WaveTimeout            int       `json:"wave_timeout_ms"`
		OutcomeSlots           [4]string `json:"outcome_slots"`
		LatestCompleteness     string    `json:"latest_completeness"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != "ardents-h3-s1-source-manifest-v1" || manifest.SourceCount != 2 ||
		manifest.RequestMagic != "ARDH3Q1\x00" || manifest.ResponseMagic != "ARDH3S1\x00" ||
		manifest.LatestOpcode != 1 || manifest.ByDigestOpcode != 2 || manifest.MaximumBundleBytes != 1<<20 ||
		manifest.MaterializationIndex != 0 || manifest.ExpectedSourceAttempts != 2 ||
		manifest.ExpectedEventKind != "source-wave-accepted" ||
		manifest.OrderSeed != "01c29b8877599dfc8dfb631880472dc0cc8becf611c0df82e5530f10a27d2ae0" ||
		manifest.ConnectTimeout != 1000 || manifest.TLSTimeout != 2000 || manifest.HeaderTimeout != 3000 ||
		manifest.TotalTimeout != 5000 || manifest.WaveTimeout != 15000 {
		t.Fatalf("unexpected finite source manifest: %+v", manifest)
	}
	if manifest.OutcomeSlots != [4]string{"latest-source-1", "latest-source-2", "by-digest-source-1", "by-digest-source-2"} ||
		manifest.LatestCompleteness != "latest completeness unproven" {
		t.Fatalf("unexpected source evidence slots: %+v", manifest)
	}
}
