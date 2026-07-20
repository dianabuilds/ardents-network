package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	maxContainerConfigBytes = 128 * 1024
	maxEnvironmentEntries   = 64
	maxEnvironmentValue     = 4 * 1024
	maxEnvironmentTotal     = 64 * 1024
	maxCommandArguments     = 64
	maxCommandBytes         = 32 * 1024
)

var environmentKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type containerResources struct {
	MemoryBytes int64 `json:"memory_bytes,omitempty"`
	NanoCPUs    int64 `json:"nano_cpus,omitempty"`
	PIDs        int64 `json:"pids,omitempty"`
	TmpfsBytes  int64 `json:"tmpfs_bytes,omitempty"`
}

type containerSpec struct {
	Image      string             `json:"image"`
	Command    []string           `json:"command,omitempty"`
	Entrypoint []string           `json:"entrypoint,omitempty"`
	Env        map[string]string  `json:"env,omitempty"`
	WorkingDir string             `json:"working_dir,omitempty"`
	User       string             `json:"user"`
	Resources  containerResources `json:"resources,omitempty"`
}

func parseContainerSpec(raw string) (containerSpec, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxContainerConfigBytes {
		return containerSpec{}, fmt.Errorf("invalid container config size")
	}
	if err := validateJSONShape([]byte(trimmed)); err != nil {
		return containerSpec{}, fmt.Errorf("invalid container config: %w", err)
	}
	var spec containerSpec
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return containerSpec{}, fmt.Errorf("invalid container config: %w", err)
	}
	if err := validateContainerSpec(&spec); err != nil {
		return containerSpec{}, err
	}
	return spec, nil
}

func ValidateContainerConfig(raw string) error {
	_, err := parseContainerSpec(raw)
	return err
}

func validateJSONShape(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := rejectDuplicateKeys(decoder, true); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("container config must contain one JSON object")
	}
	return nil
}

func rejectDuplicateKeys(decoder *json.Decoder, requireObject bool) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, composite := token.(json.Delim)
	if !composite {
		if requireObject {
			return fmt.Errorf("container config must be a JSON object")
		}
		return nil
	}
	if requireObject && delim != '{' {
		return fmt.Errorf("container config must be a JSON object")
	}
	if delim == '[' {
		return rejectDuplicateArray(decoder)
	}
	return rejectDuplicateObject(decoder)
}

func rejectDuplicateArray(decoder *json.Decoder) error {
	for decoder.More() {
		if err := rejectDuplicateKeys(decoder, false); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func rejectDuplicateObject(decoder *json.Decoder) error {
	seen := map[string]struct{}{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		name := key.(string)
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate field %q", name)
		}
		seen[name] = struct{}{}
		if err := rejectDuplicateKeys(decoder, false); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func validateContainerSpec(spec *containerSpec) error {
	if !strings.Contains(spec.Image, "@sha256:") {
		return fmt.Errorf("container image must use an immutable sha256 digest")
	}
	if err := validateContainerUser(spec.User); err != nil {
		return err
	}
	if err := validateCommand("command", spec.Command); err != nil {
		return err
	}
	if err := validateCommand("entrypoint", spec.Entrypoint); err != nil {
		return err
	}
	if err := validateWorkingDirectory(spec.WorkingDir); err != nil {
		return err
	}
	if err := validateEnvironment(spec.Env); err != nil {
		return err
	}
	return applyResourceDefaults(&spec.Resources)
}

func validateContainerUser(raw string) error {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) > 2 || len(parts) == 0 {
		return fmt.Errorf("container user must be an explicit non-root numeric identity")
	}
	for index, part := range parts {
		value, err := strconv.ParseUint(part, 10, 31)
		if err != nil || (index == 0 && value == 0) {
			return fmt.Errorf("container user must be an explicit non-root numeric identity")
		}
	}
	return nil
}

func validateCommand(field string, values []string) error {
	if len(values) > maxCommandArguments {
		return fmt.Errorf("container %s exceeds argument limit", field)
	}
	total := 0
	for _, value := range values {
		total += len(value)
		if strings.IndexByte(value, 0) >= 0 || total > maxCommandBytes {
			return fmt.Errorf("container %s exceeds size limit", field)
		}
	}
	return nil
}

func validateWorkingDirectory(value string) error {
	if value == "" {
		return nil
	}
	if !path.IsAbs(value) || path.Clean(value) != value {
		return fmt.Errorf("container working_dir must be a clean absolute path")
	}
	return nil
}

func validateEnvironment(values map[string]string) error {
	if len(values) > maxEnvironmentEntries {
		return fmt.Errorf("container environment exceeds entry limit")
	}
	total := 0
	for key, value := range values {
		if key == workloadGenerationEnvironment {
			return fmt.Errorf("container environment key is reserved")
		}
		if !environmentKeyPattern.MatchString(key) {
			return fmt.Errorf("container environment has invalid key")
		}
		if sensitiveEnvironmentKey(key) || sensitiveEnvironmentValue(value) {
			return fmt.Errorf("container environment cannot contain secret values")
		}
		total += len(key) + len(value)
		if len(value) > maxEnvironmentValue || total > maxEnvironmentTotal {
			return fmt.Errorf("container environment exceeds size limit")
		}
	}
	return nil
}

func sensitiveEnvironmentKey(key string) bool {
	normalized := strings.ToUpper(key)
	for _, suffix := range []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "PRIVATE_KEY", "CREDENTIAL"} {
		if normalized == suffix || strings.HasSuffix(normalized, "_"+suffix) {
			return true
		}
	}
	return false
}

func sensitiveEnvironmentValue(value string) bool {
	upper := strings.ToUpper(value)
	if strings.Contains(upper, "PRIVATE KEY-----") {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.User != nil
}

func encodeContainerSpec(spec containerSpec) (string, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("encode container config: %w", err)
	}
	return string(raw), nil
}

func containerEnvironment(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out
}

func runtimeEnvironment(values map[string]string, generation int64) []string {
	out := containerEnvironment(values)
	return append(out, workloadGenerationEnvironment+"="+strconv.FormatInt(generation, 10))
}
