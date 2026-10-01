//go:build linux

package installation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

// stopInstalledPredecessor is a transition phase, not Release authorization.
// Its caller holds the root mutation lease and retains its returned first error
// in the owned journal. No resource or selection publication occurs here.
func stopInstalledPredecessor(ctx context.Context, checked checkedBinding) (returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 {
		return errors.New("installed predecessor stop requires root and context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := observePlatform(ctx); err != nil {
		return err
	}
	if err := observeBinding(checked); err != nil {
		return err
	}
	return stopObservedInstallation(ctx, checked)
}

// Recovery may have torn fixed copies; the fresh-authorized intent owner checks
// their exact owned inode records before reaching this stop-only boundary.
func stopObservedInstallation(ctx context.Context, checked checkedBinding) (returnedErr error) {
	unit, service, err := worker.ReadEndpointProperties(ctx)
	if err != nil {
		return err
	}
	var pid uint32
	var invocation [16]byte
	if propertyIs(unit, "ActiveState", "s", "inactive") || propertyIs(unit, "ActiveState", "s", "failed") {
		if !propertyIs(unit, "Id", "s", "ardents-endpoint.service") || !propertyIs(unit, "LoadState", "s", "loaded") ||
			!propertyIs(unit, "FragmentPath", "s", "/etc/systemd/system/ardents-endpoint.service") || !propertyIs(unit, "DropInPaths", "as", []string{}) ||
			!propertyIs(service, "MainPID", "u", uint32(0)) || !propertyIs(service, "ExecStop", "a(sasbttttuii)", []any{}) ||
			!propertyIs(service, "ExecStopPost", "a(sasbttttuii)", []any{}) {
			return errors.New("inactive installed predecessor differs")
		}
		if err := refuseRemainingInstallationScopes(); err != nil {
			return err
		}
		if err := observeBoundSocketsForStop(ctx); err != nil {
			return err
		}
		if _, err := runInstallationManager(ctx, "--system", "--no-ask-password", "--no-pager", "stop",
			"ardents-text-reader.socket", "ardents-text-publisher.socket", "ardents-endpoint.service"); err != nil {
			return err
		}
		if err := observeStoppedInstallation(ctx); err != nil {
			return err
		}
		return refuseRemainingInstallationScopes()
	}
	if service["MainPID"].Type != "u" || json.Unmarshal(service["MainPID"].Data, &pid) != nil || pid == 0 ||
		!decodeInstalledInvocation(unit["InvocationID"], &invocation) {
		return errors.New("installed predecessor identity is unavailable")
	}
	if err := verifyInstalledProcess(unit, service, checked, pid, invocation); err != nil {
		return err
	}
	if err := observeBoundSocketsForStop(ctx); err != nil {
		return err
	}
	pins, err := pinInstallationScopes(pid, checked.binding.UID)
	if err != nil {
		return err
	}
	defer func() {
		for _, pin := range pins {
			returnedErr = errors.Join(returnedErr, pin.Close())
		}
	}()
	// Recheck the observed invocation after pinning and before requesting stop.
	unit, service, err = worker.ReadEndpointProperties(ctx)
	if err != nil {
		return err
	}
	if err := verifyInstalledProcess(unit, service, checked, pid, invocation); err != nil {
		return err
	}
	_, stopErr := runInstallationManager(ctx, "--system", "--no-ask-password", "--no-pager", "stop",
		"ardents-text-reader.socket", "ardents-text-publisher.socket", "ardents-endpoint.service")
	// Observation survives caller cancellation, but cannot erase a stop failure.
	joinCtx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	joinErr := joinInstallationPins(joinCtx, pins, worker.ReadCgroup)
	stoppedErr := observeStoppedInstallation(joinCtx)
	scopesErr := refuseRemainingInstallationScopes()
	return errors.Join(stopErr, joinErr, stoppedErr, scopesErr, ctx.Err())
}

func installedInvocation(unit, service worker.Properties) (uint32, [16]byte, error) {
	var pid uint32
	var invocation [16]byte
	if service["MainPID"].Type != "u" || json.Unmarshal(service["MainPID"].Data, &pid) != nil || pid == 0 ||
		!decodeInstalledInvocation(unit["InvocationID"], &invocation) {
		return 0, invocation, errors.New("installed invocation is unavailable")
	}
	return pid, invocation, nil
}

func decodeInstalledInvocation(value worker.Value, destination *[16]byte) bool {
	var parts []json.RawMessage
	if value.Type != "ay" || json.Unmarshal(value.Data, &parts) != nil || len(parts) != 16 {
		return false
	}
	for index, part := range parts {
		value, err := strconv.ParseUint(strings.TrimSpace(string(part)), 10, 8)
		if err != nil {
			return false
		}
		destination[index] = byte(value)
	}
	return *destination != [16]byte{}
}

func installationScopeNames() ([]string, error) {
	// Root-owned system.slice is the selected unit location; no request path
	// can redirect this inventory. Directory names never establish readiness.
	if err := checkRootAncestors("/sys/fs/cgroup/system.slice", false); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir("/sys/fs/cgroup/system.slice")
	if err != nil || len(entries) > 4096 {
		return nil, errors.New("installation scope inventory is unavailable")
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "ardents-endpoint.service" || strings.HasPrefix(name, "ardents-text-reader@") || strings.HasPrefix(name, "ardents-text-publisher@") {
			if !entry.IsDir() || len(names) >= 129 {
				return nil, errors.New("installation scope inventory is invalid")
			}
			names = append(names, name)
		}
	}
	return names, nil
}

func pinInstallationScopes(pid, uid uint32) (pins []*os.File, returnedErr error) {
	defer func() {
		if returnedErr != nil {
			for _, pin := range pins {
				returnedErr = errors.Join(returnedErr, pin.Close())
			}
			pins = nil
		}
	}()
	endpoint, err := worker.PinEndpointCgroup()
	if err != nil {
		return nil, err
	}
	pins = append(pins, endpoint)
	names, err := installationScopeNames()
	if err != nil {
		return pins, err
	}
	suffix := "-" + strconv.FormatUint(uint64(pid), 10) + "-" + strconv.FormatUint(uint64(uid), 10) + ".service"
	for _, name := range names {
		if name == "ardents-endpoint.service" {
			continue
		}
		role := "reader"
		if strings.HasPrefix(name, "ardents-text-publisher@") {
			role = "publisher"
		}
		if !worker.ValidUnit(name, role) || !strings.HasSuffix(name, suffix) {
			return pins, errors.New("installation has a foreign worker scope")
		}
		pin, err := worker.PinCgroup(worker.Instance{Name: name, Role: role, Cgroup: "/system.slice/" + name})
		if err != nil {
			return pins, err
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

func refuseRemainingInstallationScopes() error {
	names, err := installationScopeNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		var pin *os.File
		if name == "ardents-endpoint.service" {
			pin, err = worker.PinEndpointCgroup()
		} else {
			role := "reader"
			if strings.HasPrefix(name, "ardents-text-publisher@") {
				role = "publisher"
			}
			if !worker.ValidUnit(name, role) {
				return errors.New("installation scope identity is invalid")
			}
			pin, err = worker.PinCgroup(worker.Instance{Name: name, Role: role, Cgroup: "/system.slice/" + name})
		}
		if err != nil {
			return err
		}
		removed, populated, readErr := worker.ReadCgroup(pin)
		closeErr := pin.Close()
		if readErr != nil || closeErr != nil {
			return errors.Join(readErr, closeErr)
		}
		if !removed && populated {
			return errors.New("installation retains populated Endpoint or worker scopes")
		}
	}
	return nil
}

func joinInstallationPins(ctx context.Context, pins []*os.File, read func(*os.File) (bool, bool, error)) error {
	if ctx == nil || len(pins) == 0 {
		return errors.New("installation join has no original scope observations")
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		empty := true
		for _, pin := range pins {
			removed, populated, err := read(pin)
			if err != nil {
				return err
			}
			empty = empty && (removed || !populated)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if empty {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("installation original scopes did not join")
		case <-ticker.C:
		}
	}
}
