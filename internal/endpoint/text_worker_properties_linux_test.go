//go:build linux

package endpoint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTextWorkerSyscallObservationAcceptsNativeAMD64Expansion(t *testing.T) {
	// systemd reports the effective native-architecture seccomp set. Ubuntu
	// 22.04/systemd 249 on amd64 therefore omits the s390-only PCI syscalls
	// even though @raw-io names them on architectures where they exist.
	names := strings.Fields("add_key bpf chroot delete_module finit_module fsconfig fsmount fsopen fspick init_module io_uring_enter io_uring_register io_uring_setup ioperm iopl kexec_file_load kexec_load keyctl mount mount_setattr move_mount open_tree pciconfig_iobase pciconfig_read pciconfig_write perf_event_open pivot_root process_vm_readv process_vm_writev ptrace reboot request_key swapoff swapon umount umount2 userfaultfd")
	service := textManagerProperties{"SystemCallFilter": {Type: "(bas)", Data: syscallFilterObservation(t, names)}}
	if err := verifyTextWorkerSyscalls(service); err != nil {
		t.Fatalf("native amd64 deny expansion refused: %v", err)
	}

	withoutMount := append([]string(nil), names...)
	for index, name := range withoutMount {
		if name == "mount" {
			withoutMount = append(withoutMount[:index], withoutMount[index+1:]...)
			break
		}
	}
	service["SystemCallFilter"] = textManagerValue{Type: "(bas)", Data: syscallFilterObservation(t, withoutMount)}
	if err := verifyTextWorkerSyscalls(service); err == nil {
		t.Fatal("missing native mount denial accepted")
	}
}

func syscallFilterObservation(t *testing.T, names []string) json.RawMessage {
	t.Helper()
	body, err := json.Marshal([]any{false, names})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
