package unit

import (
	"errors"
	"path"
	"strconv"
	"strings"
	"unicode"
)

// EndpointTemplate retains one checked fixed Endpoint template and its bound
// command paths. It grants no Release, write-root, manager or startup authority.
type EndpointTemplate struct {
	body             string
	installationRoot string
	directory        string
}

// PrepareEndpointTemplate checks paths, the closed property contract and every
// marker before Installation separately admits its writable-root projection.
func PrepareEndpointTemplate(template []byte, installationRoot, directory string) (EndpointTemplate, error) {
	if !unitPath(directory) || !unitPath(installationRoot) ||
		path.Dir(directory) != path.Join(installationRoot, "generations") ||
		!canonicalDigest(path.Base(directory)) {
		return EndpointTemplate{}, errors.New("installation generation command path is invalid")
	}
	if len(template) == 0 || len(template) > 64<<10 {
		return EndpointTemplate{}, errors.New("installation Endpoint template exceeds its bound")
	}
	required := map[string]string{
		"Unit.Description": "Ardents protected text Endpoint",
		"Unit.Requires":    "ardents-text-reader.socket ardents-text-publisher.socket",
		"Unit.After":       "network-online.target ardents-text-reader.socket ardents-text-publisher.socket",
		"Unit.Wants":       "network-online.target",
		"Service.Type":     "exec", "Service.User": "ardents-endpoint", "Service.Group": "ardents-endpoint",
		"Service.SupplementaryGroups": "",
		"Service.ExecStart":           ":@ARDENTS_ENDPOINT_PROGRAM@ endpoint start-installed @ARDENTS_INSTALLATION_ROOT@",
		"Service.WorkingDirectory":    "/", "Service.UMask": "0077", "Service.NoNewPrivileges": "yes",
		"Service.CapabilityBoundingSet": "", "Service.AmbientCapabilities": "",
		"Service.PrivateTmp": "yes", "Service.ProtectHome": "yes", "Service.ProtectSystem": "strict",
		"Service.ReadWritePaths":       "@ARDENTS_WRITE_PATHS@",
		"Service.ProtectControlGroups": "yes", "Service.ProtectKernelTunables": "yes",
		"Service.ProtectKernelModules": "yes", "Service.ProtectKernelLogs": "yes",
		"Service.RestrictSUIDSGID": "yes", "Service.LockPersonality": "yes",
		"Service.RestrictAddressFamilies": "AF_UNIX AF_INET AF_INET6",
		"Service.MemoryAccounting":        "yes", "Service.CPUAccounting": "yes", "Service.TasksAccounting": "yes",
		"Service.LimitCORE": "0", "Service.Restart": "no", "Service.RemainAfterExit": "no",
		"Service.KillMode": "control-group", "Service.TimeoutStopSec": "30s",
		"Service.StandardOutput": "journal", "Service.StandardError": "journal",
	}
	section := ""
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(template), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if line == "[Unit]" || line == "[Service]" {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, found := strings.Cut(line, "=")
		name := section + "." + key
		expected, known := required[name]
		if !found || !known || seen[name] || value != expected {
			return EndpointTemplate{}, errors.New("installation Endpoint template differs from its fixed contract")
		}
		seen[name] = true
	}
	if len(seen) != len(required) {
		return EndpointTemplate{}, errors.New("installation Endpoint template is incomplete")
	}
	for _, marker := range []string{"@ARDENTS_ENDPOINT_PROGRAM@", "@ARDENTS_INSTALLATION_ROOT@", "@ARDENTS_WRITE_PATHS@"} {
		if strings.Count(string(template), marker) != 1 {
			return EndpointTemplate{}, errors.New("installation Endpoint marker is ambiguous")
		}
	}
	return EndpointTemplate{body: string(template), installationRoot: installationRoot, directory: directory}, nil
}

// Render encodes the caller's admitted write paths without selecting them.
func (prepared EndpointTemplate) Render(paths []string) ([]byte, error) {
	if prepared.body == "" {
		return nil, ErrInput
	}
	quoted := make([]string, len(paths))
	for index, writable := range paths {
		quoted[index] = quoteUnitPath(writable)
	}
	replacer := strings.NewReplacer(
		"@ARDENTS_ENDPOINT_PROGRAM@", quoteUnitPath(path.Join(prepared.directory, "ardents-linux-amd64")),
		"@ARDENTS_INSTALLATION_ROOT@", quoteUnitPath(prepared.installationRoot),
		"@ARDENTS_WRITE_PATHS@", strings.Join(quoted, " "))
	return []byte(replacer.Replace(prepared.body)), nil
}

func quoteUnitPath(value string) string { return strconv.Quote(strings.ReplaceAll(value, "%", "%%")) }

func unitPath(value string) bool {
	return path.IsAbs(value) && path.Clean(value) == value && !strings.ContainsAny(value, "\x00\r\n") && strings.IndexFunc(value, unicode.IsControl) == -1
}
