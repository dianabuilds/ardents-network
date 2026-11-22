//go:build linux

package worker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDocumentWorkerUnitBindingRequiresExactCollectionPolicy(t *testing.T) {
	const name = "ardents-text-reader@1-2-3.service"
	const cgroup = "/system.slice/" + name
	unit := properties{
		"Id":           {Type: "s", Data: json.RawMessage(`"` + name + `"`)},
		"LoadState":    {Type: "s", Data: json.RawMessage(`"loaded"`)},
		"ActiveState":  {Type: "s", Data: json.RawMessage(`"active"`)},
		"SubState":     {Type: "s", Data: json.RawMessage(`"running"`)},
		"FragmentPath": {Type: "s", Data: json.RawMessage(`"/etc/systemd/system/ardents-text-reader@.service"`)},
		"CollectMode":  {Type: "s", Data: json.RawMessage(`"inactive-or-failed"`)},
	}
	service := properties{"ControlGroup": {Type: "s", Data: json.RawMessage(`"` + cgroup + `"`)}}
	verified := func() bool { return workerUnitBindingVerified(textInventory, unit, service, name, "reader", cgroup) }
	if !verified() {
		t.Fatal("exact manager collection policy refused")
	}
	for _, invalid := range []value{
		{Type: "s", Data: json.RawMessage(`"inactive"`)},
		{Type: "s", Data: json.RawMessage(`"unknown"`)},
		{Type: "s", Data: json.RawMessage(`null`)},
		{Type: "as", Data: json.RawMessage(`["inactive-or-failed"]`)},
	} {
		unit["CollectMode"] = invalid
		if verified() {
			t.Fatalf("invalid manager collection policy accepted: %+v", invalid)
		}
	}
	delete(unit, "CollectMode")
	if verified() {
		t.Fatal("missing manager collection policy accepted")
	}
}

func TestDocumentWorkerSyscallObservationAcceptsNativeAMD64Expansion(t *testing.T) {
	// systemd reports the effective native-architecture seccomp set. Ubuntu
	// 24.04/systemd 255 on amd64 therefore omits the s390-only PCI syscalls
	// even though @raw-io names them on architectures where they exist.
	names := strings.Fields("add_key bpf chroot delete_module finit_module fsconfig fsmount fsopen fspick init_module io_uring_enter io_uring_register io_uring_setup ioperm iopl kexec_file_load kexec_load keyctl mount mount_setattr move_mount open_tree pciconfig_iobase pciconfig_read pciconfig_write perf_event_open pivot_root process_vm_readv process_vm_writev ptrace reboot request_key swapoff swapon umount umount2 userfaultfd")
	service := properties{"SystemCallFilter": {Type: "(bas)", Data: syscallFilterObservation(t, names)}}
	if err := verifySyscalls(service); err != nil {
		t.Fatalf("native amd64 deny expansion refused: %v", err)
	}

	withoutMount := append([]string(nil), names...)
	for index, name := range withoutMount {
		if name == "mount" {
			withoutMount = append(withoutMount[:index], withoutMount[index+1:]...)
			break
		}
	}
	service["SystemCallFilter"] = value{Type: "(bas)", Data: syscallFilterObservation(t, withoutMount)}
	if err := verifySyscalls(service); err == nil {
		t.Fatal("missing native mount denial accepted")
	}
}

func TestDocumentWorkerSliceKeepsTheClosedOrdinaryCgroupRoot(t *testing.T) {
	for _, test := range []struct {
		name  string
		value *value
		want  bool
	}{
		{name: "normal exact", value: &value{Type: "s", Data: json.RawMessage(`"system.slice"`)}, want: true},
		{name: "normal template subslice", value: &value{Type: "s", Data: json.RawMessage(`"system-ardents\\x2dtext\\x2dreader.slice"`)}},
		{name: "normal missing"},
		{name: "normal wrong type", value: &value{Type: "as", Data: json.RawMessage(`["system.slice"]`)}},
		{name: "foreign qualification slice", value: &value{Type: "s", Data: json.RawMessage(`"ardents-qualification-owner.slice"`)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := properties{}
			if test.value != nil {
				service["Slice"] = *test.value
			}
			if got := sliceVerified(textInventory, service); got != test.want {
				t.Fatalf("slice verification = %t, want %t", got, test.want)
			}
		})
	}
}

func TestStreamWorkerSliceKeepsTheQualificationOwnerSlice(t *testing.T) {
	for _, test := range []struct {
		name  string
		value *value
		want  bool
	}{
		{name: "qualification exact", value: &value{Type: "s", Data: json.RawMessage(`"ardents-qualification-owner.slice"`)}, want: true},
		{name: "ordinary system slice", value: &value{Type: "s", Data: json.RawMessage(`"system.slice"`)}},
		{name: "qualification missing"},
		{name: "qualification wrong type", value: &value{Type: "as", Data: json.RawMessage(`["ardents-qualification-owner.slice"]`)}},
		{name: "qualification template subslice", value: &value{Type: "s", Data: json.RawMessage(`"ardents-qualification-owner-ardents\\x2dstream.slice"`)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := properties{}
			if test.value != nil {
				service["Slice"] = *test.value
			}
			if got := sliceVerified(streamInventory, service); got != test.want {
				t.Fatalf("slice verification = %t, want %t", got, test.want)
			}
		})
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
