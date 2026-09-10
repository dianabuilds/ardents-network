//go:build linux

package endpoint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTextWorkerCgroupObservationNeverInfersEmptyFromMissingData(t *testing.T) {
	for _, body := range []string{"", "populated 0\n", "frozen 0\n", "populated 0\npopulated 0\n", "populated 0\nfrozen 0\nextra 0\n",
		"populated 2\nfrozen 0\n", "populated 0\nfrozen null\n", strings.Repeat(" ", 1025) + "populated 0 frozen 0"} {
		if _, err := decodeTextWorkerCgroup([]byte(body)); err == nil {
			t.Fatalf("unknown cgroup observation accepted: %q", body[:min(len(body), 80)])
		}
	}
	for _, body := range []string{"populated 0\nfrozen 0\n", "frozen 1\npopulated 0\n"} {
		if populated, err := decodeTextWorkerCgroup([]byte(body)); err != nil || populated {
			t.Fatalf("known empty cgroup: populated=%v err=%v", populated, err)
		}
	}
	if populated, err := decodeTextWorkerCgroup([]byte("populated 1\nfrozen 0\n")); err != nil || !populated {
		t.Fatal("live descendants were classified as empty")
	}
}

func TestTextWorkerCgroupPathRejectsTraversalAndForeignUnits(t *testing.T) {
	name := "ardents-text-reader@0-12-997.service"
	for _, group := range []string{"", "/" + name, "/system.slice/../" + name, "/system.slice//" + name,
		"/system.slice/" + name + "/child", "/system.slice/other.service", "/system.slice/\n" + name} {
		if textWorkerCgroupPath(group, name, "reader") {
			t.Fatalf("foreign cgroup path accepted: %q", group)
		}
	}
	if !textWorkerCgroupPath("/system.slice/system-ardents\\x2dtext\\x2dreader.slice/"+name, name, "reader") {
		t.Fatal("installed worker slice path refused")
	}
}

func TestTextWorkerCleanupRejectsChangedOrUnknownInvocation(t *testing.T) {
	instance, unit, service := textCleanupObservation(t)
	if !sameTextWorkerCleanupInstance(instance, unit, service) {
		t.Fatal("exact live cleanup invocation refused")
	}
	for _, test := range []struct {
		unit                bool
		key, signature, raw string
	}{
		{true, "InvocationID", "ay", `[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`},
		{true, "LoadState", "s", `"not-found"`},
		{true, "Id", "s", `"ardents-text-reader@1-12-997.service"`},
		{false, "MainPID", "u", `99`}, {false, "MainPID", "u", ` null `},
		{false, "ControlGroup", "s", ` null `}, {false, "ControlGroup", "s", `"/system.slice/foreign.service"`},
		{false, "Delegate", "b", `true`}, {false, "Delegate", "b", `null`},
		{false, "KillMode", "s", `"process"`}, {false, "TimeoutStopUSec", "t", `0`},
		{false, "Restart", "s", `"always"`}, {false, "ExecStop", "a(sasbttttuii)", `null`},
	} {
		instance, unit, service := textCleanupObservation(t)
		properties := service
		if test.unit {
			properties = unit
		}
		properties[test.key] = textManagerValue{Type: test.signature, Data: json.RawMessage(test.raw)}
		if sameTextWorkerCleanupInstance(instance, unit, service) {
			t.Fatalf("cleanup accepted altered %s", test.key)
		}
	}
	service["MainPID"] = textManagerValue{Type: "u", Data: json.RawMessage(`0`)}
	service["ControlGroup"] = textManagerValue{Type: "s", Data: json.RawMessage(`""`)}
	if !sameTextWorkerCleanupInstance(instance, unit, service) {
		t.Fatal("same invocation after initial PID exit was lost")
	}
}

func textCleanupObservation(t *testing.T) (textWorkerInstance, textManagerProperties, textManagerProperties) {
	t.Helper()
	instance := textWorkerInstance{name: "ardents-text-reader@0-12-997.service", role: "reader", pid: 42, uid: 61234, invocation: [16]byte{1}}
	instance.cgroup = "/system.slice/" + instance.name
	value := func(signature string, data any) textManagerValue {
		body, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		return textManagerValue{Type: signature, Data: body}
	}
	unit := textManagerProperties{"Id": value("s", instance.name), "LoadState": value("s", "loaded"),
		"InvocationID": value("ay", []int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})}
	service := textManagerProperties{"MainPID": value("u", instance.pid), "ControlGroup": value("s", instance.cgroup),
		"Restart": value("s", "no"), "KillMode": value("s", "control-group"), "Delegate": value("b", false),
		"TimeoutStopUSec": value("t", uint64(2_000_000)), "ExecStop": value("a(sasbttttuii)", []any{}), "ExecStopPost": value("a(sasbttttuii)", []any{})}
	return instance, unit, service
}

func TestTextWorkerCleanupFailureCannotBecomeSuccessOnRepeat(t *testing.T) {
	owner := &textWorkerCleanup{}
	first := owner.Close()
	if first == nil || owner.Close() != first {
		t.Fatal("missing cleanup identity became reusable after failure")
	}
}
