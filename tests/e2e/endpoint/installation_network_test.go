package endpoint

import (
	"encoding/json"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/plan"
)

func TestInstallationNetworkKeepsPinsAndPathsAsLiteralInputs(t *testing.T) {
	pin := strings.Repeat("ab", 32)
	base := path.Join("/fixture", strings.Repeat("02", 32))
	bundle := path.Join(base, "bundle")
	input := InstallationNetwork{
		Base: base, InstallationRoot: path.Join(base, "installed"), StateRoot: path.Join(base, "state"),
		ClockFile: path.Join(base, "clock"), ClientCertificate: path.Join(base, "client.pem"), ClientKey: path.Join(base, "client.key"),
		Network: [32]byte{1}, Authority: [32]byte{2}, Seed: [32]byte{3},
		Addresses: [2]string{"127.0.0.1:10001", "127.0.0.1:10002"}, SourcePins: [2][32]byte{{4}, {5}},
		ObservedAt: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC), RefreshMilliseconds: 60000,
	}
	raw := InstallationRequest(t, bundle, pin, path.Join(base, "floors"))
	var got struct {
		Bundle   string `json:"bundle_root"`
		Pin      string `json:"manifest_sha256"`
		Headless struct {
			StateRoot string `json:"network_state_root"`
		} `json:"headless"`
		Source struct {
			Refresh int `json:"refresh_interval_ms"`
			Sources []struct {
				Address string `json:"address"`
				RawPin  string `json:"leaf_key_digest"`
			} `json:"sources"`
		} `json:"source"`
	}
	bound := []byte(BindInstallationNetwork(t, string(raw), input))
	if _, err := plan.Decode(t.Context(), bound, true); err != nil {
		t.Fatalf("Endpoint request rejected: %v", err)
	}
	if err := json.Unmarshal(bound, &got); err != nil {
		t.Fatal(err)
	}
	if got.Bundle != bundle || got.Pin != pin || got.Headless.StateRoot != input.StateRoot ||
		got.Source.Refresh != 60000 || len(got.Source.Sources) != len(input.Addresses) {
		t.Fatalf("installation fixture changed unrelated literals or lost host inputs: %+v", got)
	}
	for i, source := range got.Source.Sources {
		if source.Address != input.Addresses[i] || source.RawPin != []string{"04", "05"}[i]+strings.Repeat("00", 31) {
			t.Fatalf("Source %d fields crossed: %+v", i, source)
		}
	}
}
