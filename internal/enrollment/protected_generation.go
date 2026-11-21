package enrollment

import (
	"bytes"
	"encoding/json"
)

type protectedDescriptor struct {
	Schema          string            `json:"schema"`
	Platform        string            `json:"platform"`
	ReleaseIdentity string            `json:"release_identity"`
	ReleaseVersion  int64             `json:"release_version"`
	Files           map[string]string `json:"files"`
}

func protectedNames() []string {
	return []string{"ardents-linux-amd64", "ardents-text-linux-amd64", "ardents-text-reader@.service",
		"ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket",
		"50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"}
}

// ValidateProtectedGeneration checks only the canonical static resource group.
// It supplies no initial pin provenance, Release proof or installed authority.
func ValidateProtectedGeneration(descriptor []byte, resources map[string][]byte, releaseIdentity string) error {
	if len(resources) != len(protectedNames()) {
		return ErrInventory
	}
	files := make(map[string][]byte, len(resources)+1)
	for name, body := range resources {
		files[name] = body
	}
	files["protected-endpoint.json"] = descriptor
	protected, err := verifyProtected(files, "linux-amd64", "ardents-linux-amd64", releaseIdentity)
	if err != nil {
		return err
	}
	if !protected {
		return ErrInventory
	}
	return nil
}

func verifyProtected(files map[string][]byte, platform, artifact, release string) (bool, error) {
	reserved := append([]string{"protected-endpoint.json"}, protectedNames()[1:]...)
	present := 0
	for _, name := range reserved {
		if _, ok := files[name]; ok {
			present++
		}
	}
	if present == 0 {
		return false, nil
	}
	if present != len(reserved) {
		return false, ErrInventory
	}
	if platform != "linux-amd64" || artifact != "ardents-linux-amd64" {
		return false, ErrBinding
	}
	raw := files["protected-endpoint.json"]
	if len(raw) > 16<<10 {
		return false, ErrInventory
	}
	var generation protectedDescriptor
	if err := json.Unmarshal(raw, &generation); err != nil {
		return false, ErrInventory
	}
	canonical, err := json.Marshal(generation)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return false, ErrInventory
	}
	if generation.Schema != "ardents-protected-endpoint-artifact-v1" || generation.Platform != platform ||
		generation.ReleaseIdentity != release || generation.ReleaseVersion < 1 || len(generation.Files) != 9 {
		return false, ErrBinding
	}
	for _, name := range protectedNames() {
		if !matchesDigest(files[name], generation.Files[name]) {
			return false, ErrBinding
		}
	}
	return true, nil
}
