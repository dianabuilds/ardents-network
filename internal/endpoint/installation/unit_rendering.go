package installation

import (
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func renderEndpointUnit(template []byte, request Request, directory string) ([]byte, error) {
	if !unitPath(directory) || !unitPath(request.InstallationRoot) {
		return nil, errors.New("installation Endpoint command path is invalid")
	}
	if len(template) == 0 || len(template) > 64<<10 {
		return nil, errors.New("installation Endpoint unit template exceeds its bound")
	}
	required := map[string]string{
		"Unit.Description": "Ardents protected text Endpoint", "Unit.Requires": "ardents-text-reader.socket ardents-text-publisher.socket",
		"Unit.After": "network-online.target ardents-text-reader.socket ardents-text-publisher.socket", "Unit.Wants": "network-online.target",
		"Service.Type": "exec", "Service.User": "ardents-endpoint", "Service.Group": "ardents-endpoint", "Service.SupplementaryGroups": "",
		"Service.ExecStart":        ":@ARDENTS_ENDPOINT_PROGRAM@ endpoint start-installed @ARDENTS_INSTALLATION_ROOT@",
		"Service.WorkingDirectory": "/", "Service.UMask": "0077", "Service.NoNewPrivileges": "yes", "Service.CapabilityBoundingSet": "", "Service.AmbientCapabilities": "",
		"Service.PrivateTmp": "yes", "Service.ProtectHome": "yes", "Service.ProtectSystem": "strict", "Service.ReadWritePaths": "@ARDENTS_WRITE_PATHS@",
		"Service.ProtectControlGroups": "yes", "Service.ProtectKernelTunables": "yes", "Service.ProtectKernelModules": "yes", "Service.ProtectKernelLogs": "yes",
		"Service.RestrictSUIDSGID": "yes", "Service.LockPersonality": "yes", "Service.RestrictAddressFamilies": "AF_UNIX AF_INET AF_INET6",
		"Service.MemoryAccounting": "yes", "Service.CPUAccounting": "yes", "Service.TasksAccounting": "yes", "Service.LimitCORE": "0",
		"Service.Restart": "no", "Service.RemainAfterExit": "no", "Service.ExitType": "main", "Service.RestartMode": "normal",
		"Service.KillMode": "control-group", "Service.TimeoutStopSec": "30s", "Service.StandardOutput": "journal", "Service.StandardError": "journal",
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
			return nil, errors.New("installation Endpoint unit template does not match its fixed contract")
		}
		seen[name] = true
	}
	if len(seen) != len(required) {
		return nil, errors.New("installation Endpoint unit template is incomplete")
	}
	for _, marker := range []string{"@ARDENTS_ENDPOINT_PROGRAM@", "@ARDENTS_INSTALLATION_ROOT@", "@ARDENTS_WRITE_PATHS@"} {
		if strings.Count(string(template), marker) != 1 {
			return nil, errors.New("installation Endpoint unit marker is ambiguous")
		}
	}
	paths := writableDirectories(request)
	var quoted []string
	for _, path := range paths {
		if !unitPath(path) {
			return nil, errors.New("installation writable unit path is invalid")
		}
		quoted = append(quoted, quoteUnitPath(path))
	}
	replacer := strings.NewReplacer("@ARDENTS_ENDPOINT_PROGRAM@", quoteUnitPath(filepath.Join(directory, "ardents-linux-amd64")),
		"@ARDENTS_INSTALLATION_ROOT@", quoteUnitPath(request.InstallationRoot), "@ARDENTS_WRITE_PATHS@", strings.Join(quoted, " "))
	return []byte(replacer.Replace(string(template))), nil
}

func writableDirectories(request Request) []string {
	paths := mutableRoots(request.Headless)
	for _, path := range []string{request.Headless.ApplicationSocket, request.Headless.ReaderPermission.RequestPath, request.Headless.ReaderPermission.ResponsePath} {
		paths = append(paths, filepath.Dir(path))
	}
	if request.Headless.Role == "" {
		for _, path := range []string{request.Headless.AdministrationSocket, request.Headless.PublisherPermission.RequestPath, request.Headless.PublisherPermission.ResponsePath} {
			paths = append(paths, filepath.Dir(path))
		}
	}
	sort.Strings(paths)
	var distinct []string
	previous := ""
	for _, path := range paths {
		if path != previous {
			distinct = append(distinct, path)
			previous = path
		}
	}
	return distinct
}

func quoteUnitPath(path string) string { return strconv.Quote(strings.ReplaceAll(path, "%", "%%")) }

func unitPath(path string) bool {
	return canonicalPath(path) && strings.IndexFunc(path, unicode.IsControl) == -1
}
