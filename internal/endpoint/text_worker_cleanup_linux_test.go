//go:build linux

package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
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
	if !textWorkerCgroupPath("/system.slice/"+name, name, "reader") {
		t.Fatal("installed worker cgroup path refused")
	}
	if textWorkerCgroupPath("/system.slice/foreign.scope/"+name, name, "reader") {
		t.Fatal("nested foreign text worker cgroup accepted")
	}
	streamName := "ardents-stream-qualification-reader@0-12-997.service"
	if !textWorkerCgroupPath(streamWorkerCgroupRoot+streamName, streamName, "reader") {
		t.Fatal("qualification owner slice path refused")
	}
	for _, group := range []string{"/system.slice/" + streamName, "/foreign" + streamWorkerCgroupRoot + streamName, streamWorkerCgroupRoot + "nested/" + streamName} {
		if textWorkerCgroupPath(group, streamName, "reader") {
			t.Fatalf("foreign qualification cgroup path accepted: %q", group)
		}
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

func TestTextWorkerCleanupAcceptsRetiredOrEmptyPinnedCgroupAfterInvocationMismatch(t *testing.T) {
	for name, final := range map[string]struct{ removed, populated bool }{
		"removed": {removed: true}, "empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			instance, unit, service := textCleanupObservation(t)
			unit["InvocationID"] = textManagerValue{Type: "ay", Data: json.RawMessage(`[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}
			observations := []struct {
				removed, populated bool
				err                error
			}{{populated: true}, {removed: final.removed, populated: final.populated}}
			owner := &textWorkerCleanup{
				instance: instance,
				readCgroup: func(*os.File) (bool, bool, error) {
					observation := observations[0]
					observations = observations[1:]
					return observation.removed, observation.populated, observation.err
				},
				readProperties: func(context.Context, string, string) (textManagerProperties, textManagerProperties, error) {
					return unit, service, nil
				},
				joinTimeout: time.Second,
				stop: func(context.Context, string, string) error {
					t.Fatal("cleanup tried to stop a changed invocation")
					return nil
				},
			}
			if err := owner.join(); err != nil {
				t.Fatalf("retired or empty pinned cgroup after changed invocation = %v", err)
			}
		})
	}
}

func TestTextWorkerCleanupRetainsInvocationMismatchWhilePinnedCgroupPopulated(t *testing.T) {
	instance, unit, service := textCleanupObservation(t)
	unit["InvocationID"] = textManagerValue{Type: "ay", Data: json.RawMessage(`[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}
	owner := &textWorkerCleanup{
		instance:   instance,
		readCgroup: func(*os.File) (bool, bool, error) { return false, true, nil },
		readProperties: func(context.Context, string, string) (textManagerProperties, textManagerProperties, error) {
			return unit, service, nil
		},
		joinTimeout: time.Millisecond,
		stop: func(context.Context, string, string) error {
			t.Fatal("cleanup tried to stop a changed invocation")
			return nil
		},
	}
	if err := owner.join(); err == nil || err.Error() != "text worker cleanup invocation changed" {
		t.Fatalf("populated pinned cgroup lost invocation mismatch: %v", err)
	}
}

func TestTextWorkerCleanupRetainsInitialPinnedCgroupObservationFailure(t *testing.T) {
	instance, unit, service := textCleanupObservation(t)
	unit["InvocationID"] = textManagerValue{Type: "ay", Data: json.RawMessage(`[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}
	initial := errors.New("pinned observation failed")
	owner := &textWorkerCleanup{
		instance:    instance,
		readCgroup:  func(*os.File) (bool, bool, error) { return false, false, initial },
		joinTimeout: time.Second,
		readProperties: func(context.Context, string, string) (textManagerProperties, textManagerProperties, error) {
			return unit, service, nil
		},
	}
	if err := owner.join(); !errors.Is(err, initial) {
		t.Fatalf("initial pinned cgroup failure was replaced: %v", err)
	}
}

func TestTextWorkerCleanupRejectsRetiredPinnedCgroupAfterJoinDeadline(t *testing.T) {
	instance, unit, service := textCleanupObservation(t)
	unit["InvocationID"] = textManagerValue{Type: "ay", Data: json.RawMessage(`[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}
	observations := []struct {
		removed, populated bool
	}{{populated: true}, {removed: true}}
	owner := &textWorkerCleanup{
		instance: instance,
		readCgroup: func(*os.File) (bool, bool, error) {
			observation := observations[0]
			observations = observations[1:]
			return observation.removed, observation.populated, nil
		},
		readProperties: func(ctx context.Context, _ string, _ string) (textManagerProperties, textManagerProperties, error) {
			<-ctx.Done()
			return unit, service, nil
		},
		joinTimeout: time.Millisecond,
	}
	if err := owner.join(); err == nil || err.Error() != "text worker cleanup invocation changed" {
		t.Fatalf("expired cleanup accepted retired cgroup: %v", err)
	}
}
