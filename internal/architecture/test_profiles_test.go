package architecture

import (
	"encoding/json"
	"strings"
	"testing"
)

type testProfileRegistry struct {
	Schema   string        `json:"schema"`
	Profiles []testProfile `json:"profiles"`
}

type testProfile struct {
	ID                 string   `json:"id"`
	MakeTarget         string   `json:"make_target"`
	Surface            string   `json:"surface"`
	Prerequisites      []string `json:"prerequisites"`
	InvalidEnvironment string   `json:"invalid_environment"`
}

func TestTestProfileRegistryIsFactualAndWired(t *testing.T) {
	root := repositoryRoot(t)
	data := readProjectFile(t, root, "tests/profiles/profiles.json")
	var registry testProfileRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("decode test profile registry: %v", err)
	}
	if registry.Schema != "ardents-test-profiles-v1" {
		t.Fatalf("profile registry schema = %q", registry.Schema)
	}
	makefile := string(readProjectFile(t, root, "Makefile"))
	required := map[string]bool{
		"developer":               false,
		"deterministic":           false,
		"process":                 false,
		"race":                    false,
		"live":                    false,
		"historical-reproduction": false,
	}
	for _, profile := range registry.Profiles {
		if _, known := required[profile.ID]; !known {
			t.Errorf("unknown test profile %q", profile.ID)
			continue
		}
		if required[profile.ID] {
			t.Errorf("duplicate test profile %q", profile.ID)
		}
		required[profile.ID] = true
		if profile.MakeTarget == "" || !strings.Contains(makefile, "\n"+profile.MakeTarget+":") {
			t.Errorf("profile %q names absent Make target %q", profile.ID, profile.MakeTarget)
		}
		if profile.Surface == "" || profile.InvalidEnvironment == "" {
			t.Errorf("profile %q lacks surface or invalid-environment result", profile.ID)
		}
		if profile.InvalidEnvironment == "not applicable" && len(profile.Prerequisites) != 0 {
			t.Errorf("profile %q cannot have prerequisites when invalid environment is not applicable", profile.ID)
		}
	}
	for id, found := range required {
		if !found {
			t.Errorf("required test profile %q is missing", id)
		}
	}
}
