//go:build linux

package endpoint

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const textWorkerRoot = "/usr/lib/ardents/text-worker-root"

// verifyTextWorkerProperties is only one input to a qualified launch receipt.
// The caller must separately establish the pinned artifact, accepted socket's
// kernel credentials, live process/cgroup identity and joined cleanup owner.
func verifyTextWorkerProperties(unit, service textManagerProperties, name, role, cgroup string, pid uint32) error {
	if !textWorkerCgroupPath(cgroup, name, role) || pid == 0 {
		return errors.New("text worker process binding is invalid")
	}
	for key, want := range map[string]string{"Id": name, "LoadState": "loaded", "ActiveState": "active", "SubState": "running",
		"FragmentPath": "/etc/systemd/system/ardents-text-" + role + "@.service", "ControlGroup": cgroup, "CollectMode": "inactive-or-failed"} {
		// ControlGroup is a Service property; Unit owns the remaining identity.
		properties := unit
		if key == "ControlGroup" {
			properties = service
		}
		if !properties.exact(key, "s", want) {
			return errors.New("text worker unit binding is unavailable")
		}
	}
	if !unit.exact("BindsTo", "as", []string{"ardents-endpoint.service"}) || !textUnitAfterEndpoint(unit) {
		return errors.New("text worker Endpoint lifetime binding is unavailable")
	}
	if !service.exact("Delegate", "b", false) || !unit.exact("DropInPaths", "as", []string{}) || !unit.exact("JoinsNamespaceOf", "as", []string{}) || !service.exact("MainPID", "u", pid) {
		return errors.New("text worker unit composition is unavailable")
	}
	for _, key := range []string{"DynamicUser", "NoNewPrivileges", "PrivateNetwork", "PrivateIPC", "PrivateDevices", "PrivateTmp",
		"ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryDenyWriteExecute"} {
		if !service.exact(key, "b", true) {
			return errors.New("text worker required hardening is unavailable")
		}
	}
	userPrefix := "ardtxt-r-"
	if role == "publisher" {
		userPrefix = "ardtxt-p-"
	}
	user := userPrefix + strings.TrimSuffix(strings.TrimPrefix(name, "ardents-text-"+role+"@"), ".service")
	for key, want := range map[string]string{"User": user, "Group": user, "RootDirectory": textWorkerRoot, "WorkingDirectory": "/",
		"ProtectSystem": "strict", "ProtectHome": "yes", "Restart": "no", "KillMode": "control-group",
		"StandardInput": "socket", "StandardOutput": "socket", "StandardError": "null", "NotifyAccess": "none",
		"RootImage": "", "NetworkNamespacePath": "", "IPCNamespacePath": "", "PAMName": ""} {
		if !service.exact(key, "s", want) {
			return errors.New("text worker required boundary is unavailable")
		}
	}
	for key, want := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "RestrictNamespaces": 0,
		"MemoryMax": 128 << 20, "TasksMax": 32, "LimitCORE": 0, "LimitCORESoft": 0, "TimeoutStopUSec": 2_000_000} {
		if !service.exact(key, "t", want) {
			return errors.New("text worker resource or privilege bound is unavailable")
		}
	}
	for key, signature := range map[string]string{"SupplementaryGroups": "as", "ReadWritePaths": "as", "BindPaths": "a(ssbt)",
		"BindReadOnlyPaths": "a(ssbt)", "LoadCredential": "a(ss)", "LoadCredentialEncrypted": "a(ss)",
		"SetCredential": "a(say)", "SetCredentialEncrypted": "a(say)", "ImportCredential": "as", "EnvironmentFiles": "a(sb)",
		"PassEnvironment": "as", "ExecStartPre": "a(sasbttttuii)", "ExecStartPost": "a(sasbttttuii)",
		"ExecStop": "a(sasbttttuii)", "ExecStopPost": "a(sasbttttuii)", "ExecCondition": "a(sasbttttuii)", "ExecReload": "a(sasbttttuii)"} {
		if !service.exact(key, signature, []any{}) {
			return errors.New("text worker inherited resources are unavailable")
		}
	}
	if !service.exact("Environment", "as", []string{"GOMAXPROCS=2", "GOMEMLIMIT=96MiB"}) ||
		!service.exact("SystemCallArchitectures", "as", []string{"native"}) ||
		!service.exact("RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_UNIX"}}) {
		return errors.New("text worker execution policy is unavailable")
	}
	if err := verifyTextWorkerExec(service, role, pid); err != nil {
		return err
	}
	return verifyTextWorkerSyscalls(service)
}

func verifyTextWorkerExec(service textManagerProperties, role string, pid uint32) error {
	value, ok := service["ExecStartEx"]
	var entries [][]json.RawMessage
	if !ok || value.Type != "a(sasasttttuii)" || json.Unmarshal(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return errors.New("text worker executable binding is unavailable")
	}
	parts := entries[0]
	var program string
	var arguments []string
	var observedPID uint32
	if json.Unmarshal(parts[0], &program) != nil || program != "/ardents-text" || json.Unmarshal(parts[1], &arguments) != nil ||
		len(arguments) != 2 || arguments[0] != program || arguments[1] != "worker-"+role ||
		strings.TrimSpace(string(parts[2])) != "[]" || json.Unmarshal(parts[7], &observedPID) != nil || observedPID != pid {
		return errors.New("text worker executable binding is unavailable")
	}
	// A populated monotonic start observation excludes an unstarted template.
	started, err := strconv.ParseUint(string(parts[4]), 10, 64)
	if err != nil || started == 0 {
		return errors.New("text worker executable has not started")
	}
	return nil
}

func verifyTextWorkerSyscalls(service textManagerProperties) error {
	value, ok := service["SystemCallFilter"]
	var parts []json.RawMessage
	var names []string
	if !ok || value.Type != "(bas)" || json.Unmarshal(value.Data, &parts) != nil || len(parts) != 2 ||
		strings.TrimSpace(string(parts[0])) != "false" || json.Unmarshal(parts[1], &names) != nil {
		return errors.New("text worker syscall policy is unavailable")
	}
	denied := make(map[string]bool, len(names))
	for _, name := range names {
		denied[name] = true
	}
	// systemd 255's selected deny groups are checked after expansion. Merely
	// finding a group name in a unit file is not evidence of effective seccomp.
	for _, name := range strings.Fields("add_key bpf chroot delete_module finit_module fsconfig fsmount fsopen fspick init_module io_uring_enter io_uring_register io_uring_setup ioperm iopl kexec_file_load kexec_load keyctl mount mount_setattr move_mount open_tree pciconfig_iobase pciconfig_read pciconfig_write perf_event_open pivot_root process_vm_readv process_vm_writev ptrace reboot request_key s390_pci_mmio_read s390_pci_mmio_write swapoff swapon umount umount2 userfaultfd") {
		if !denied[name] {
			return errors.New("text worker syscall denial is unavailable")
		}
	}
	return nil
}
