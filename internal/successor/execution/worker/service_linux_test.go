//go:build linux

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestAwaitTextEndpointServiceObservationRetriesTransientFailure(t *testing.T) {
	attempts := 0
	err := awaitServiceObservation(t.Context(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("transient manager observation")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("got %d observations, want 3", attempts)
	}
}

func TestAwaitTextEndpointServiceObservationRemainsFailClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Millisecond)
	defer cancel()
	attempts := 0
	err := awaitServiceObservation(ctx, func(context.Context) error {
		attempts++
		return errors.New("permanent manager observation failure")
	})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want bounded fail-closed result", err)
	}
	if attempts < 2 {
		t.Fatalf("got %d observations, want a retry", attempts)
	}
}

func TestOrdinaryExecutionRefusesOlderManagerEvenWithMainExitInvariant(t *testing.T) {
	service := properties{"RemainAfterExit": {Type: "b", Data: []byte("false")}}
	if endpointStopsWithMain(service, 249) {
		t.Fatal("ordinary Execution admitted an unselected manager")
	}
	for _, version := range []uint16{0, 248, 250, 255} {
		if endpointStopsWithMain(service, version) {
			t.Fatalf("accepted missing properties on %d", version)
		}
	}
	service["ExitType"] = value{Type: "s", Data: []byte("\"cgroup\"")}
	if endpointStopsWithMain(service, 249) {
		t.Fatal("unexpected cgroup lifetime accepted")
	}
	delete(service, "ExitType")
	service["RemainAfterExit"] = value{Type: "b", Data: []byte("true")}
	if endpointStopsWithMain(service, 249) {
		t.Fatal("retained parent accepted")
	}
}

func TestEndpointMainExitCannotLeaveItsUnitLogicallyActive(t *testing.T) {
	valid := properties{
		"RemainAfterExit": {Type: "b", Data: json.RawMessage("false")},
		"ExitType":        {Type: "s", Data: json.RawMessage(`"main"`)},
		"RestartMode":     {Type: "s", Data: json.RawMessage(`"normal"`)},
	}
	if !endpointStopsWithMain(valid, 255) {
		t.Fatal("selected Endpoint lifetime refused")
	}
	for _, test := range []struct {
		name  string
		value value
	}{
		{"RemainAfterExit", value{Type: "b", Data: json.RawMessage("true")}},
		{"RemainAfterExit", value{Type: "b", Data: json.RawMessage("null")}},
		{"ExitType", value{Type: "s", Data: json.RawMessage(`"cgroup"`)}},
		{"RestartMode", value{Type: "s", Data: json.RawMessage(`"direct"`)}},
		{"RestartMode", value{}},
	} {
		previous := valid[test.name]
		valid[test.name] = test.value
		if endpointStopsWithMain(valid, 255) {
			t.Errorf("accepted parent lifetime %s=%s", test.name, test.value.Data)
		}
		valid[test.name] = previous
	}
}

func TestTextLaunchRejectsOtherOrUnknownPlatform(t *testing.T) {
	if !ubuntuRelease([]byte("ID=ubuntu\nVERSION_ID=\"24.04\"\n")) {
		t.Fatal("selected OS identity refused")
	}
	for _, body := range []string{"ID=ubuntu\nVERSION_ID=22.04\n", "ID=debian\nVERSION_ID=24.04\n", "ID=ubuntu\nVERSION_ID=26.04\n", "ID=ubuntu\n", "ID=ubuntu\nID=debian\nVERSION_ID=24.04\n"} {
		if ubuntuRelease([]byte(body)) {
			t.Fatal("unsupported or ambiguous OS admitted")
		}
	}
	for _, test := range []struct {
		value value
		want  bool
	}{
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"255.4-1ubuntu8.17"}]`)}, true},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"249.11"}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"256.4"}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"2550.4"}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"u","data":255}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`null`)}, false},
	} {
		if managerVersionValid(test.value) != test.want {
			t.Fatal("unknown system manager version admitted or selected version refused")
		}
	}
}
