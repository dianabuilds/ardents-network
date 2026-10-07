package installation

import (
	"errors"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// renderEndpointUnit is a byte mechanism inside generation preparation. Its
// output grants no native ownership, manager admission or running invocation.
// Native preparation must establish those facts before publishing a selection.
func renderEndpointUnit(template []byte, request installationRequest, directory string) ([]byte, error) {
	if !unitPath(directory) || !unitPath(request.InstallationRoot) ||
		path.Dir(directory) != path.Join(request.InstallationRoot, "generations") ||
		!canonicalDigest(path.Base(directory)) {
		return nil, errors.New("installation generation command path is invalid")
	}
	if len(template) == 0 || len(template) > 64<<10 {
		return nil, errors.New("installation Endpoint template exceeds its bound")
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
			return nil, errors.New("installation Endpoint template differs from its fixed contract")
		}
		seen[name] = true
	}
	if len(seen) != len(required) {
		return nil, errors.New("installation Endpoint template is incomplete")
	}
	for _, marker := range []string{"@ARDENTS_ENDPOINT_PROGRAM@", "@ARDENTS_INSTALLATION_ROOT@", "@ARDENTS_WRITE_PATHS@"} {
		if strings.Count(string(template), marker) != 1 {
			return nil, errors.New("installation Endpoint marker is ambiguous")
		}
	}
	paths, err := writableDirectories(request)
	if err != nil {
		return nil, err
	}
	quoted := make([]string, len(paths))
	for index, writable := range paths {
		quoted[index] = quoteUnitPath(writable)
	}
	replacer := strings.NewReplacer(
		"@ARDENTS_ENDPOINT_PROGRAM@", quoteUnitPath(path.Join(directory, "ardents-linux-amd64")),
		"@ARDENTS_INSTALLATION_ROOT@", quoteUnitPath(request.InstallationRoot),
		"@ARDENTS_WRITE_PATHS@", strings.Join(quoted, " "))
	return []byte(replacer.Replace(string(template))), nil
}

// These are the actual write allowances rendered into the fixed unit. Include
// socket/permission parent directories, rather than checking only their leaves:
// a writable parent must not enclose immutable artifacts or retained trust.
func writableDirectories(request installationRequest) ([]string, error) {
	paths := mutableRoots(request.Headless)
	for _, file := range []string{request.Headless.ApplicationSocket, request.Headless.ReaderPermission.RequestPath, request.Headless.ReaderPermission.ResponsePath} {
		paths = append(paths, path.Dir(file))
	}
	if request.Headless.Role == "" {
		for _, file := range []string{request.Headless.AdministrationSocket, request.Headless.PublisherPermission.RequestPath, request.Headless.PublisherPermission.ResponsePath} {
			paths = append(paths, path.Dir(file))
		}
	}
	for _, writable := range paths {
		if !unitPath(writable) || writable == "/" {
			return nil, errors.New("installation writable directory is invalid")
		}
		for _, immutable := range []string{request.BundleRoot, request.InstallationRoot, request.ReleaseFloorRoot} {
			if pathsOverlap(writable, immutable) {
				return nil, errors.New("installation write allowance overlaps immutable or Release roots")
			}
		}
	}
	sort.Strings(paths)
	distinct := make([]string, 0, len(paths))
	for _, writable := range paths {
		if len(distinct) == 0 || writable != distinct[len(distinct)-1] {
			distinct = append(distinct, writable)
		}
	}
	return distinct, nil
}

func quoteUnitPath(value string) string { return strconv.Quote(strings.ReplaceAll(value, "%", "%%")) }

func unitPath(value string) bool {
	return canonicalPath(value) && strings.IndexFunc(value, unicode.IsControl) == -1
}
