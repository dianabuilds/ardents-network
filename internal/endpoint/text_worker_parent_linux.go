//go:build linux

package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
)

// The calling Endpoint must be the installed system service's main process.
// BindsTo plus After ties each worker to loss of this service, including an
// uncatchable Endpoint exit. Socket EOF alone cannot terminate a hostile fork.
func verifyTextEndpointService(ctx context.Context) error {
	if err := verifyTextWorkerPlatform(ctx); err != nil {
		return err
	}
	answer, err := textManagerCall(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager", "GetUnit", "s", "ardents-endpoint.service")
	var paths []string
	if err != nil || answer.Type != "o" || json.Unmarshal(answer.Data, &paths) != nil || len(paths) != 1 {
		return errors.New("text worker Endpoint service is unavailable")
	}
	for _, kind := range []string{"Unit", "Service"} {
		answer, err := textManagerCall(ctx, paths[0], "org.freedesktop.DBus.Properties", "GetAll", "s", "org.freedesktop.systemd1."+kind)
		var properties []textManagerProperties
		if err != nil || answer.Type != "a{sv}" || json.Unmarshal(answer.Data, &properties) != nil || len(properties) != 1 {
			return errors.New("text worker Endpoint service properties are unavailable")
		}
		if kind == "Unit" {
			if !properties[0].exact("Id", "s", "ardents-endpoint.service") || !properties[0].exact("ActiveState", "s", "active") {
				return errors.New("text worker Endpoint service is not active")
			}
		} else if os.Geteuid() == 0 || !properties[0].exact("MainPID", "u", uint32(os.Getpid())) ||
			!properties[0].exact("User", "s", "ardents-endpoint") || !properties[0].exact("Group", "s", "ardents-endpoint") ||
			!textEndpointStopsWithMain(properties[0]) {
			return errors.New("text worker caller is not the Endpoint service")
		}
	}
	return nil
}

func textUnitAfterEndpoint(unit textManagerProperties) bool {
	value, ok := unit["After"]
	var units []string
	if !ok || value.Type != "as" || json.Unmarshal(value.Data, &units) != nil {
		return false
	}
	for _, name := range units {
		if name == "ardents-endpoint.service" {
			return true
		}
	}
	return false
}

func textEndpointStopsWithMain(service textManagerProperties) bool {
	return service.exact("RemainAfterExit", "b", false) && service.exact("ExitType", "s", "main") &&
		service.exact("RestartMode", "s", "normal")
}

func verifyTextWorkerPlatform(ctx context.Context) error {
	body, err := readTextInstalledFile("/usr/lib/os-release", 8<<10)
	if err != nil || runtime.GOARCH != "amd64" || !textUbuntuRelease(body) {
		return errors.New("text worker Ubuntu platform is unavailable")
	}
	answer, err := textManagerCall(ctx, "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version")
	if err != nil || !textSystemdVersion(answer) {
		return errors.New("text worker systemd version is unavailable")
	}
	return nil
}

func textUbuntuRelease(body []byte) bool {
	id, version := "", ""
	idSeen, versionSeen := false, false
	for _, line := range strings.Split(string(body), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		if key == "ID" {
			if idSeen {
				return false
			}
			id, idSeen = value, true
		}
		if key == "VERSION_ID" {
			if versionSeen {
				return false
			}
			version, versionSeen = value, true
		}
	}
	return (id == "ubuntu" || id == `"ubuntu"`) && (version == "24.04" || version == `"24.04"`)
}

func textSystemdVersion(answer textManagerValue) bool {
	var variants []textManagerValue
	var version string
	return answer.Type == "v" && json.Unmarshal(answer.Data, &variants) == nil && len(variants) == 1 &&
		variants[0].Type == "s" && json.Unmarshal(variants[0].Data, &version) == nil &&
		(version == "255" || strings.HasPrefix(version, "255."))
}
