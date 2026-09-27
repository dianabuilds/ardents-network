//go:build linux

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"time"
)

// The calling Endpoint must be the installed system service's main process.
// BindsTo plus After ties each worker to loss of this service, including an
// uncatchable Endpoint exit. Socket EOF alone cannot terminate a hostile fork.
func VerifyEndpointService(ctx context.Context) error {
	if ctx == nil {
		return errors.New("text worker Endpoint service is unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return awaitServiceObservation(bounded, observeEndpointService)
}

func awaitServiceObservation(ctx context.Context, observe func(context.Context) error) error {
	if ctx == nil || observe == nil {
		return errors.New("text worker Endpoint service is unavailable")
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last error
	for {
		if err := observe(ctx); err == nil {
			return nil
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return errors.Join(last, ctx.Err())
		case <-ticker.C:
		}
	}
}

func observeEndpointService(ctx context.Context) error {
	if err := verifyPlatform(ctx); err != nil {
		return err
	}
	version, err := ManagerVersion(ctx)
	if err != nil {
		return err
	}
	answer, err := managerCall(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager", "GetUnit", "s", "ardents-endpoint.service")
	var paths []string
	if err != nil || answer.Type != "o" || json.Unmarshal(answer.Data, &paths) != nil || len(paths) != 1 {
		return errors.New("text worker Endpoint service is unavailable")
	}
	for _, kind := range []string{"Unit", "Service"} {
		answer, err := managerCall(ctx, paths[0], "org.freedesktop.DBus.Properties", "GetAll", "s", "org.freedesktop.systemd1."+kind)
		var properties []Properties
		if err != nil || answer.Type != "a{sv}" || json.Unmarshal(answer.Data, &properties) != nil || len(properties) != 1 {
			return errors.New("text worker Endpoint service properties are unavailable")
		}
		if kind == "Unit" {
			if !properties[0].exact("Id", "s", "ardents-endpoint.service") || !properties[0].exact("ActiveState", "s", "active") {
				return errors.New("text worker Endpoint service is not active")
			}
		} else if os.Geteuid() == 0 || !properties[0].exact("MainPID", "u", uint32(os.Getpid())) ||
			!properties[0].exact("User", "s", "ardents-endpoint") || !properties[0].exact("Group", "s", "ardents-endpoint") ||
			!endpointStopsWithMain(properties[0], version) {
			return errors.New("text worker caller is not the Endpoint service")
		}
	}
	return nil
}

func unitAfterEndpoint(unit Properties) bool {
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

func endpointStopsWithMain(service Properties, version uint16) bool {
	if version != 249 && version != 255 || !service.exact("RemainAfterExit", "b", false) {
		return false
	}
	for key, want := range map[string]string{"ExitType": "main", "RestartMode": "normal"} {
		_, present := service[key]
		// v249 has neither selectable cgroup exit nor direct restart modes.
		// Only this observed manager version permits those absent properties.
		if !present && version == 249 {
			continue
		}
		if !service.exact(key, "s", want) {
			return false
		}
	}
	return true
}

func verifyPlatform(ctx context.Context) error {
	body, err := ReadInstalledFile("/usr/lib/os-release", 8<<10)
	if err != nil || runtime.GOARCH != "amd64" || !ubuntuRelease(body) {
		return errors.New("text worker Ubuntu platform is unavailable")
	}
	answer, err := managerCall(ctx, "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version")
	if err != nil || !managerVersionValid(answer) {
		return errors.New("text worker systemd version is unavailable")
	}
	return nil
}

func ubuntuRelease(body []byte) bool {
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
	id, version = strings.Trim(id, "\""), strings.Trim(version, "\"")
	return id == "ubuntu" && (version == "22.04" || version == "24.04")
}

func managerVersionValid(answer Value) bool { return decodeManagerVersion(answer) != 0 }

func decodeManagerVersion(answer Value) uint16 {
	var variants []Value
	var version string
	if answer.Type != "v" || json.Unmarshal(answer.Data, &variants) != nil || len(variants) != 1 ||
		variants[0].Type != "s" || json.Unmarshal(variants[0].Data, &version) != nil {
		return 0
	}
	for _, major := range []struct {
		text  string
		value uint16
	}{{"249", 249}, {"255", 255}} {
		if version == major.text || strings.HasPrefix(version, major.text+".") {
			return major.value
		}
	}
	return 0
}

func ManagerVersion(ctx context.Context) (uint16, error) {
	answer, err := managerCall(ctx, "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version")
	if err != nil {
		return 0, err
	}
	version := decodeManagerVersion(answer)
	if version == 0 {
		return 0, errors.New("text worker systemd version unavailable")
	}
	return version, nil
}
