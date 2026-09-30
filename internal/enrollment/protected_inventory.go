package enrollment

import (
	"bytes"
	"encoding/json"
	"errors"
)

type protectedDescriptor struct {
	Schema          string            `json:"schema"`
	Platform        string            `json:"platform"`
	ReleaseIdentity string            `json:"release_identity"`
	ReleaseVersion  int64             `json:"release_version"`
	Files           map[string]string `json:"files"`
}

// ValidateProtectedGeneration checks producer bytes with the same grammar and
// resource binding as enrollment. It does not authenticate a manifest, Release
// metadata or execution authority; the caller must separately obtain those.
func ValidateProtectedGeneration(raw []byte, resources map[string][]byte, releaseIdentity string) error {
	for _, contents := range resources {
		if len(contents) == 0 || len(contents) > maximumFileLen {
			return errors.New("protected generation resource exceeds its bound")
		}
	}
	if len(resources) != 9 {
		return errors.New("protected generation resource inventory is invalid")
	}
	if len(raw) > 16<<10 {
		return errors.New("protected generation descriptor exceeds its bound")
	}
	var generation protectedDescriptor
	if err := json.Unmarshal(raw, &generation); err != nil {
		return errors.New("protected generation descriptor is invalid")
	}
	canonical, err := json.Marshal(generation)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("protected generation descriptor is not canonical")
	}
	if generation.Schema != "ardents-protected-endpoint-artifact-v1" || generation.Platform != "linux-amd64" ||
		generation.ReleaseIdentity != releaseIdentity || generation.ReleaseVersion < 1 || len(generation.Files) != 9 {
		return errors.New("protected generation descriptor identity or inventory is invalid")
	}
	for _, name := range ProtectedResourceNames() {
		contents, present := resources[name]
		if !present || !equalDigest(contents, generation.Files[name]) {
			return errors.New("protected generation resource does not match descriptor")
		}
	}
	return nil
}

// ProtectedResourceNames returns the exact protected generation inventory.
// Each call returns a fresh slice; callers cannot alter the grammar owner.
func ProtectedResourceNames() []string {
	return []string{"ardents-linux-amd64", "ardents-text-linux-amd64",
		"ardents-text-reader@.service", "ardents-text-publisher@.service",
		"ardents-text-reader.socket", "ardents-text-publisher.socket",
		"50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"}
}

// projectProtectedInventory reserves the whole protected companion group at
// every enrollment entry point. The existing Endpoint executable is already
// mandatory; it is not a signal that an old general bundle is protected.
func projectProtectedInventory(files map[string][]byte, enrolled descriptor) (map[string][]byte, error) {
	names := append([]string{"protected-endpoint.json"}, ProtectedResourceNames()[1:]...)
	present := 0
	for _, name := range names {
		if _, found := files[name]; found {
			present++
		}
	}
	if present == 0 {
		return nil, nil
	}
	if present != len(names) {
		return nil, errors.New("alpha enrollment has a partial protected companion inventory")
	}
	if enrolled.platform != "linux-amd64" || enrolled.artifact != "ardents-linux-amd64" {
		return nil, errors.New("protected companion inventory requires linux-amd64 Endpoint")
	}
	resources := make(map[string][]byte, 9)
	for _, name := range ProtectedResourceNames() {
		resources[name] = files[name]
	}
	if err := ValidateProtectedGeneration(files["protected-endpoint.json"], resources, enrolled.release); err != nil {
		return nil, err
	}
	for name, contents := range resources {
		resources[name] = append([]byte(nil), contents...)
	}
	return resources, nil
}
