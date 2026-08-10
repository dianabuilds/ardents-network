package tooling

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"regexp"
)

const nativeToolRoleSchema = "carrier-lab-native-tool-role/v1"

var nativeLinkName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)

type nativeToolConfig struct {
	SchemaVersion     string              `json:"schema_version"`
	RunID             string              `json:"run_id"`
	Role              string              `json:"role"`
	Mode              string              `json:"mode"`
	DelayMilliseconds int                 `json:"delay_milliseconds,omitempty"`
	Links             []nativeCaptureLink `json:"links,omitempty"`
}

type nativeCaptureLink struct {
	Name string `json:"name"`
	Peer string `json:"peer"`
}

// RunNativeRole executes one native-route shaping or capture sidecar from a
// fixed data-only configuration. It remains laboratory tooling, not a Route
// dependency.
func RunNativeRole(configPath, evidenceDirectory, captureDirectory string) error {
	config, err := readNativeToolConfig(configPath)
	if err != nil {
		return err
	}
	for name, path := range map[string]string{"evidence": evidenceDirectory, "capture": captureDirectory} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return errors.New(name + " directory must already exist")
		}
	}
	if config.Mode == "shape" {
		return runNativeShaper(config, evidenceDirectory)
	}
	return runNativeCapture(config, evidenceDirectory, captureDirectory)
}

func readNativeToolConfig(path string) (nativeToolConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nativeToolConfig{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config nativeToolConfig
	if err := decoder.Decode(&config); err != nil || decoder.More() {
		return nativeToolConfig{}, errors.New("native tooling configuration has invalid encoding")
	}
	if config.SchemaVersion != nativeToolRoleSchema || !runIDPattern.MatchString(config.RunID) || !nativeLinkName.MatchString(config.Role) {
		return nativeToolConfig{}, errors.New("native tooling identity is invalid")
	}
	if config.Mode == "shape" {
		if config.DelayMilliseconds != 0 && config.DelayMilliseconds != 40 || len(config.Links) != 0 {
			return nativeToolConfig{}, errors.New("native shaping configuration is outside the fixed impairment")
		}
		return config, nil
	}
	if config.Mode != "capture" || config.DelayMilliseconds != 0 || len(config.Links) == 0 {
		return nativeToolConfig{}, errors.New("native tooling mode or capture links are invalid")
	}
	seen := make(map[string]bool, len(config.Links))
	for _, link := range config.Links {
		if !nativeLinkName.MatchString(link.Name) || !nativeLinkName.MatchString(link.Peer) || seen[link.Name] {
			return nativeToolConfig{}, errors.New("native capture link is invalid or duplicated")
		}
		seen[link.Name] = true
	}
	return config, nil
}
