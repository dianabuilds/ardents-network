//go:build linux

package installation

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

func verifyInstalledProcess(ctx context.Context, unit, service worker.Properties, checked checkedBinding, pid uint32, invocation [16]byte) error {
	version, err := worker.ManagerVersion(ctx)
	if err != nil {
		return err
	}
	return verifyInstalledProcessVersion(version, unit, service, checked, pid, invocation)
}

func verifyInstalledProcessVersion(version uint16, unit, service worker.Properties, checked checkedBinding, pid uint32, invocation [16]byte) error {
	if version != 249 && version != 255 {
		return errors.New("installed Endpoint manager version is unavailable")
	}
	if pid == 0 || invocation == [16]byte{} {
		return errors.New("installed process identity is incomplete")
	}
	for key, want := range map[string]string{"Id": "ardents-endpoint.service", "LoadState": "loaded", "ActiveState": "active", "SubState": "running", "FragmentPath": "/etc/systemd/system/ardents-endpoint.service"} {
		if !propertyIs(unit, key, "s", want) {
			return errors.New("installed Endpoint unit identity differs")
		}
	}
	if !propertyIs(unit, "DropInPaths", "as", []string{}) || !propertyIs(unit, "InvocationID", "ay", invocation) {
		return errors.New("installed Endpoint invocation or drop-ins differ")
	}
	for key, want := range map[string]string{"User": "ardents-endpoint", "Group": "ardents-endpoint", "Type": "exec", "ControlGroup": "/system.slice/ardents-endpoint.service", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no"} {
		if !propertyIs(service, key, "s", want) {
			return errors.New("installed Endpoint service contract differs")
		}
	}
	for key, want := range map[string]string{"ExitType": "main", "RestartMode": "normal"} {
		// Only the observed 249 manager lacks these selectable policies. Its
		// parent lifetime remains mandatory through RemainAfterExit=false.
		if _, present := service[key]; !present && version == 249 {
			continue
		}
		if !propertyIs(service, key, "s", want) {
			return errors.New("installed Endpoint parent lifetime differs")
		}
	}
	if !propertyIs(service, "MainPID", "u", pid) || !propertyIs(service, "UMask", "u", uint32(0077)) ||
		!propertyIs(service, "CapabilityBoundingSet", "t", uint64(0)) || !propertyIs(service, "AmbientCapabilities", "t", uint64(0)) ||
		!propertyIs(service, "LimitCORE", "t", uint64(0)) || !propertyIs(service, "SupplementaryGroups", "as", []string{}) ||
		!propertyIs(service, "ReadWritePaths", "as", writableDirectories(checked.request)) {
		return errors.New("installed Endpoint identity, capabilities or writable paths differ")
	}
	for _, key := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !propertyIs(service, key, "b", true) {
			return errors.New("installed Endpoint protection is unavailable")
		}
	}
	if !propertyIs(service, "RemainAfterExit", "b", false) {
		return errors.New("installed Endpoint lifetime differs")
	}
	for _, key := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !propertyIs(service, key, "a(sasbttttuii)", []any{}) {
			return errors.New("installed Endpoint has additional execution commands")
		}
	}
	if err := verifyInstalledExec(service, checked, pid); err != nil {
		return err
	}
	addressFamilies, present := service["RestrictAddressFamilies"]
	var familyParts []json.RawMessage
	var allow bool
	var families []string
	if !present || addressFamilies.Type != "(bas)" || json.Unmarshal(addressFamilies.Data, &familyParts) != nil || len(familyParts) != 2 ||
		json.Unmarshal(familyParts[0], &allow) != nil || !allow || json.Unmarshal(familyParts[1], &families) != nil {
		return errors.New("installed Endpoint address-family protection is unavailable")
	}
	slices.Sort(families)
	if !slices.Equal(families, []string{"AF_INET", "AF_INET6", "AF_UNIX"}) {
		return errors.New("installed Endpoint address-family policy differs")
	}
	return nil
}

func propertyIs(properties worker.Properties, key, signature string, expected any) bool {
	property, present := properties[key]
	if !present || property.Type != signature {
		return false
	}
	wanted, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	var actualValue, expectedValue any
	return json.Unmarshal(property.Data, &actualValue) == nil && json.Unmarshal(wanted, &expectedValue) == nil && reflect.DeepEqual(actualValue, expectedValue)
}

func verifyInstalledExec(service worker.Properties, checked checkedBinding, pid uint32) error {
	property, present := service["ExecStartEx"]
	var entries [][]json.RawMessage
	if !present || property.Type != "a(sasasttttuii)" || json.Unmarshal(property.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return errors.New("installed executable observation is unavailable")
	}
	parts := entries[0]
	var program string
	var arguments, flags []string
	var started uint64
	var observedPID uint32
	wantedProgram := filepath.Join(checked.directory, "ardents-linux-amd64")
	if json.Unmarshal(parts[0], &program) != nil || program != wantedProgram || json.Unmarshal(parts[1], &arguments) != nil ||
		!slices.Equal(arguments, []string{wantedProgram, "endpoint", "start-installed", checked.binding.InstallationRoot}) ||
		json.Unmarshal(parts[2], &flags) != nil || !slices.Equal(flags, []string{"no-env-expand"}) ||
		json.Unmarshal(parts[4], &started) != nil || started == 0 || json.Unmarshal(parts[7], &observedPID) != nil || observedPID != pid {
		return errors.New("installed executable, arguments, flags or main process differ")
	}
	return nil
}
