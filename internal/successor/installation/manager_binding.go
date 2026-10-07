package installation

import (
	"bytes"
	"encoding/json"
	"io"
	"path"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"
)

type managerValue struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type managerProperties map[string]managerValue

// Preserve signatures, integer precision and absence. Duplicate fields cannot
// replace an earlier required observation, even inside a variant dictionary.
func decodeManagerJSON(raw []byte, target any) error {
	if len(raw) == 0 || len(raw) > 64<<10 || !utf8.Valid(raw) {
		return ErrBinding
	}
	tokens := json.NewDecoder(bytes.NewReader(raw))
	tokens.UseNumber()
	if err := observeJSONValue(tokens, 0); err != nil {
		return err
	}
	if _, err := tokens.Token(); err != io.EOF {
		return ErrBinding
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return ErrBinding
	}
	return nil
}

func observeJSONValue(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return ErrBinding
	}
	token, err := decoder.Token()
	if err != nil {
		return ErrBinding
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return ErrBinding
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			name, stringKey := key.(string)
			if err != nil || !stringKey || seen[name] {
				return ErrBinding
			}
			seen[name] = true
		}
		if err := observeJSONValue(decoder, depth+1); err != nil {
			return err
		}
	}
	closeToken, err := decoder.Token()
	wanted := json.Delim(']')
	if delimiter == '{' {
		wanted = '}'
	}
	if err != nil || closeToken != wanted {
		return ErrBinding
	}
	return nil
}

func managerPropertyMatches(properties managerProperties, name, signature string, wanted any) bool {
	value, found := properties[name]
	if !found || value.Type != signature {
		return false
	}
	encoded, err := json.Marshal(wanted)
	if err != nil {
		return false
	}
	var actual, expected any
	if decodeManagerJSON(value.Data, &actual) != nil || decodeManagerJSON(encoded, &expected) != nil {
		return false
	}
	return reflect.DeepEqual(actual, expected)
}

func managerPropertyContains(properties managerProperties, name string, required []string) bool {
	value, present := properties[name]
	var items []string
	if !present || value.Type != "as" || decodeManagerJSON(value.Data, &items) != nil || items == nil {
		return false
	}
	seen := map[string]bool{}
	for _, item := range items {
		if seen[item] {
			return false
		}
		seen[item] = true
	}
	for _, item := range required {
		if !seen[item] {
			return false
		}
	}
	return true
}

// Stopped configuration admission is not a running MainPID/InvocationID proof,
// a qualified worker or capability-profile repair. Both known manager profiles
// retain their current parent-lifetime refusal; no newer profile is admitted.
func verifyStoppedEndpointProperties(version string, unit, service managerProperties, request installationRequest, generationDigest string) error {
	major, _, _ := strings.Cut(version, ".")
	if major != "249" && major != "255" || !canonicalDigest(generationDigest) {
		return ErrNativeUnavailable
	}
	for name, wanted := range map[string]string{"Id": "ardents-endpoint.service", "LoadState": "loaded", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "/etc/systemd/system/ardents-endpoint.service"} {
		if !managerPropertyMatches(unit, name, "s", wanted) {
			return ErrBinding
		}
	}
	if !managerPropertyMatches(unit, "DropInPaths", "as", []string{}) ||
		!managerPropertyContains(unit, "Requires", []string{"ardents-text-reader.socket", "ardents-text-publisher.socket"}) {
		return ErrBinding
	}
	for name, wanted := range map[string]string{"User": "ardents-endpoint", "Group": "ardents-endpoint", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": ""} {
		if !managerPropertyMatches(service, name, "s", wanted) {
			return ErrBinding
		}
	}
	for name, wanted := range map[string]string{"ExitType": "main", "RestartMode": "normal"} {
		if _, present := service[name]; !present && major == "249" {
			continue
		}
		if !managerPropertyMatches(service, name, "s", wanted) {
			return ErrBinding
		}
	}
	paths, err := writableDirectories(request)
	if err != nil {
		return err
	}
	if !managerPropertyMatches(service, "MainPID", "u", uint32(0)) || !managerPropertyMatches(service, "UMask", "u", uint32(0077)) ||
		!managerPropertyMatches(service, "SupplementaryGroups", "as", []string{}) || !managerPropertyMatches(service, "ReadWritePaths", "as", paths) {
		return ErrBinding
	}
	for _, name := range []string{"CapabilityBoundingSet", "AmbientCapabilities", "LimitCORE"} {
		if !managerPropertyMatches(service, name, "t", uint64(0)) {
			return ErrBinding
		}
	}
	if !managerPropertyMatches(service, "TimeoutStopUSec", "t", uint64(30_000_000)) {
		return ErrBinding
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !managerPropertyMatches(service, name, "b", true) {
			return ErrBinding
		}
	}
	if !managerPropertyMatches(service, "RemainAfterExit", "b", false) {
		return ErrBinding
	}
	for _, name := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !managerPropertyMatches(service, name, "a(sasbttttuii)", []any{}) {
			return ErrBinding
		}
	}
	families, found := service["RestrictAddressFamilies"]
	var parts []json.RawMessage
	var allow bool
	var names []string
	if !found || families.Type != "(bas)" || decodeManagerJSON(families.Data, &parts) != nil || len(parts) != 2 ||
		decodeManagerJSON(parts[0], &allow) != nil || !allow || decodeManagerJSON(parts[1], &names) != nil {
		return ErrBinding
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"AF_INET", "AF_INET6", "AF_UNIX"}) {
		return ErrBinding
	}
	return verifyStoppedEndpointExec(service, request.InstallationRoot, generationDigest)
}

func verifyStoppedEndpointExec(properties managerProperties, root, generationDigest string) error {
	value, found := properties["ExecStartEx"]
	var entries [][]json.RawMessage
	if !found || value.Type != "a(sasasttttuii)" || decodeManagerJSON(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return ErrBinding
	}
	program := path.Join(root, "generations", generationDigest, "ardents-linux-amd64")
	for index, wanted := range map[int]any{0: program, 1: []string{program, "endpoint", "start-installed", root}, 2: []string{"no-env-expand"}} {
		var actual, expected any
		encoded, err := json.Marshal(wanted)
		if err != nil || decodeManagerJSON(entries[0][index], &actual) != nil || decodeManagerJSON(encoded, &expected) != nil || !reflect.DeepEqual(actual, expected) {
			return ErrBinding
		}
	}
	// Fresh initial provisioning has never started this command. Retained process
	// timestamps or a nonzero PID are not evidence of a stopped fresh operation.
	for _, part := range entries[0][3:] {
		var number uint64
		if decodeManagerJSON(part, &number) != nil || number != 0 {
			return ErrBinding
		}
	}
	return nil
}
