//go:build linux

package worker

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
		if _, err := decodeCgroupEvents([]byte(body)); err == nil {
			t.Fatalf("unknown cgroup observation accepted: %q", body[:min(len(body), 80)])
		}
	}
	for _, body := range []string{"populated 0\nfrozen 0\n", "frozen 1\npopulated 0\n"} {
		if populated, err := decodeCgroupEvents([]byte(body)); err != nil || populated {
			t.Fatalf("known empty cgroup: populated=%v err=%v", populated, err)
		}
	}
	if populated, err := decodeCgroupEvents([]byte("populated 1\nfrozen 0\n")); err != nil || !populated {
		t.Fatal("live descendants were classified as empty")
	}
}

func TestTextWorkerCgroupPathRejectsTraversalAndForeignUnits(t *testing.T) {
	name := "ardents-text-reader@0-12-997.service"
	for _, group := range []string{"", "/" + name, "/system.slice/../" + name, "/system.slice//" + name,
		"/system.slice/" + name + "/child", "/system.slice/other.service", "/system.slice/\n" + name} {
		if cgroupPath(group, name, "reader") {
			t.Fatalf("foreign cgroup path accepted: %q", group)
		}
	}
	if !cgroupPath("/system.slice/"+name, name, "reader") {
		t.Fatal("installed worker cgroup path refused")
	}
	if cgroupPath("/system.slice/foreign.scope/"+name, name, "reader") {
		t.Fatal("nested foreign text worker cgroup accepted")
	}
	streamName := "ardents-stream-qualification-reader@0-12-997.service"
	if !cgroupPath(streamCgroupRoot+streamName, streamName, "reader") {
		t.Fatal("qualification owner slice path refused")
	}
	for _, group := range []string{"/system.slice/" + streamName, "/foreign" + streamCgroupRoot + streamName, streamCgroupRoot + "nested/" + streamName} {
		if cgroupPath(group, streamName, "reader") {
			t.Fatalf("foreign qualification cgroup path accepted: %q", group)
		}
	}
}

func TestTextWorkerCleanupRejectsChangedOrUnknownInvocation(t *testing.T) {
	instance, unit, service := cleanupObservation(t)
	if !sameCleanupInstance(instance, unit, service) {
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
		instance, unit, service := cleanupObservation(t)
		properties := service
		if test.unit {
			properties = unit
		}
		properties[test.key] = Value{Type: test.signature, Data: json.RawMessage(test.raw)}
		if sameCleanupInstance(instance, unit, service) {
			t.Fatalf("cleanup accepted altered %s", test.key)
		}
	}
	service["MainPID"] = Value{Type: "u", Data: json.RawMessage(`0`)}
	service["ControlGroup"] = Value{Type: "s", Data: json.RawMessage(`""`)}
	if !sameCleanupInstance(instance, unit, service) {
		t.Fatal("same invocation after initial PID exit was lost")
	}
}

func cleanupObservation(t *testing.T) (Instance, Properties, Properties) {
	t.Helper()
	instance := Instance{Name: "ardents-text-reader@0-12-997.service", Role: "reader", PID: 42, UID: 61234, Invocation: [16]byte{1}}
	instance.Cgroup = "/system.slice/" + instance.Name
	value := func(signature string, data any) Value {
		body, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		return Value{Type: signature, Data: body}
	}
	unit := Properties{"Id": value("s", instance.Name), "LoadState": value("s", "loaded"),
		"InvocationID": value("ay", []int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})}
	service := Properties{"MainPID": value("u", instance.PID), "ControlGroup": value("s", instance.Cgroup),
		"Restart": value("s", "no"), "KillMode": value("s", "control-group"), "Delegate": value("b", false),
		"TimeoutStopUSec": value("t", uint64(2_000_000)), "ExecStop": value("a(sasbttttuii)", []any{}), "ExecStopPost": value("a(sasbttttuii)", []any{})}
	return instance, unit, service
}

func TestTextWorkerCleanupFailureCannotBecomeSuccessOnRepeat(t *testing.T) {
	owner := &Cleanup{}
	first := owner.Close()
	if first == nil || owner.Close() != first {
		t.Fatal("missing cleanup identity became reusable after failure")
	}
}

func TestTextWorkerCleanupJoinsOriginalCgroupAfterInvocationChange(t *testing.T) {
	instance, unit, service := cleanupObservation(t)
	unit["InvocationID"] = Value{Type: "ay", Data: json.RawMessage(`[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}
	owner := &Cleanup{instance: instance}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reads, stops := 0, 0
	err := owner.joinObserved(ctx,
		func(*os.File) (bool, bool, error) {
			reads++
			return false, reads == 1, nil
		},
		func(context.Context, string, string) (Properties, Properties, error) {
			return unit, service, nil
		},
		func(context.Context, string, string) error {
			stops++
			return nil
		})
	if err == nil || err.Error() != "text worker cleanup invocation changed" {
		t.Fatalf("changed invocation result: %v", err)
	}
	if reads < 2 || stops != 0 {
		t.Fatalf("original cgroup observations=%d, replacement stops=%d", reads, stops)
	}
}

func TestTextWorkerCleanupJoinsOriginalCgroupWhenManagerUnavailable(t *testing.T) {
	instance, _, _ := cleanupObservation(t)
	owner := &Cleanup{instance: instance}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reads, stops := 0, 0
	err := owner.joinObserved(ctx,
		func(*os.File) (bool, bool, error) {
			reads++
			return false, reads < 3, nil
		},
		func(context.Context, string, string) (Properties, Properties, error) {
			return nil, nil, errors.New("manager unavailable")
		},
		func(context.Context, string, string) error {
			stops++
			return nil
		})
	if err == nil || err.Error() != "text worker cleanup invocation is unavailable" {
		t.Fatalf("unavailable manager result: %v", err)
	}
	if reads < 3 || stops != 0 {
		t.Fatalf("original cgroup observations=%d, replacement stops=%d", reads, stops)
	}
}

func TestTextWorkerCleanupAcceptsOriginalCgroupRemovalWithoutManager(t *testing.T) {
	instance, _, _ := cleanupObservation(t)
	owner := &Cleanup{instance: instance}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reads := 0
	err := owner.joinObserved(ctx,
		func(*os.File) (bool, bool, error) {
			reads++
			return reads == 2, reads == 1, nil
		},
		func(context.Context, string, string) (Properties, Properties, error) {
			return nil, nil, errors.New("manager unavailable")
		},
		func(context.Context, string, string) error {
			t.Fatal("replacement stop requested")
			return nil
		})
	if err != nil || reads != 2 {
		t.Fatalf("removed original cgroup: observations=%d, result=%v", reads, err)
	}
}

func TestTextWorkerCleanupUnknownOriginalObservationFails(t *testing.T) {
	instance, unit, service := cleanupObservation(t)
	unit["InvocationID"] = Value{Type: "ay", Data: json.RawMessage(`[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}
	owner := &Cleanup{instance: instance}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	unknown := errors.New("unknown pinned cgroup observation")
	reads := 0
	err := owner.joinObserved(ctx,
		func(*os.File) (bool, bool, error) {
			reads++
			if reads > 1 {
				return false, false, unknown
			}
			return false, true, nil
		},
		func(context.Context, string, string) (Properties, Properties, error) {
			return unit, service, nil
		},
		func(context.Context, string, string) error {
			t.Fatal("replacement stop requested")
			return nil
		})
	if !errors.Is(err, unknown) || reads != 2 {
		t.Fatalf("unknown original cgroup: observations=%d, result=%v", reads, err)
	}
}

func TestTextWorkerCleanupLiveOriginalCgroupHasFiniteJoin(t *testing.T) {
	instance, unit, service := cleanupObservation(t)
	unit["InvocationID"] = Value{Type: "ay", Data: json.RawMessage(`[2,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}
	owner := &Cleanup{instance: instance}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()
	reads := 0
	err := owner.joinObserved(ctx,
		func(*os.File) (bool, bool, error) {
			reads++
			return false, true, nil
		},
		func(context.Context, string, string) (Properties, Properties, error) {
			return unit, service, nil
		},
		func(context.Context, string, string) error {
			t.Fatal("replacement stop requested")
			return nil
		})
	if err == nil || !strings.Contains(err.Error(), "text worker cgroup cleanup did not complete") || reads < 2 {
		t.Fatalf("live original cgroup: observations=%d, result=%v", reads, err)
	}
}
