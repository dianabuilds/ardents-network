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

func TestEndpointMainExitSupportsSelectedManagers(t *testing.T) {
	// 249 has no named ExitType/RestartMode properties; their absence with
	// RemainAfterExit=false is the default main-exit behaviour and is admitted,
	// mirroring the unit contract's 249 tolerance.
	base249 := properties{"RemainAfterExit": {Type: "b", Data: []byte("false")}}
	if !endpointStopsWithMain(base249, 249) {
		t.Fatal("selected 249 main-exit lifetime refused")
	}
	valid := properties{
		"RemainAfterExit": {Type: "b", Data: json.RawMessage("false")},
		"ExitType":        {Type: "s", Data: json.RawMessage(`"main"`)},
		"RestartMode":     {Type: "s", Data: json.RawMessage(`"normal"`)},
	}
	if !endpointStopsWithMain(valid, 255) {
		t.Fatal("selected 255 main-exit lifetime refused")
	}
	if !endpointStopsWithMain(valid, 249) {
		t.Fatal("249 with correct named properties refused")
	}
	// Unselected managers refuse even with the main-exit invariant.
	for _, version := range []uint16{0, 248, 250, 256} {
		if endpointStopsWithMain(base249, version) {
			t.Fatalf("unselected manager %d admitted", version)
		}
	}
	// 255 requires the named properties; absence refuses.
	if endpointStopsWithMain(base249, 255) {
		t.Fatal("255 admitted without mandatory named properties")
	}
	// Wrong present values refuse on both selected managers.
	for _, version := range []uint16{249, 255} {
		bad := properties{
			"RemainAfterExit": {Type: "b", Data: json.RawMessage("false")},
			"ExitType":        {Type: "s", Data: json.RawMessage(`"cgroup"`)},
			"RestartMode":     {Type: "s", Data: json.RawMessage(`"normal"`)},
		}
		if endpointStopsWithMain(bad, version) {
			t.Fatalf("cgroup lifetime accepted on %d", version)
		}
		retained := properties{
			"RemainAfterExit": {Type: "b", Data: json.RawMessage("true")},
			"ExitType":        {Type: "s", Data: json.RawMessage(`"main"`)},
			"RestartMode":     {Type: "s", Data: json.RawMessage(`"normal"`)},
		}
		if endpointStopsWithMain(retained, version) {
			t.Fatalf("retained parent accepted on %d", version)
		}
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

func TestTextLaunchPlatformAdmission(t *testing.T) {
	// Both admitted Ubuntu releases are accepted; the worker is not tied to one OS.
	for _, body := range []string{"ID=ubuntu\nVERSION_ID=\"24.04\"\n", "ID=ubuntu\nVERSION_ID=\"22.04\"\n", "ID=ubuntu\nVERSION_ID=22.04\n"} {
		if !ubuntuRelease([]byte(body)) {
			t.Fatalf("selected OS identity refused: %q", body)
		}
	}
	for _, body := range []string{"ID=debian\nVERSION_ID=24.04\n", "ID=ubuntu\nVERSION_ID=26.04\n", "ID=ubuntu\nVERSION_ID=20.04\n", "ID=ubuntu\n", "ID=ubuntu\nID=debian\nVERSION_ID=24.04\n", "ID=ubuntu\nVERSION_ID=24.04\nVERSION_ID=22.04\n"} {
		if ubuntuRelease([]byte(body)) {
			t.Fatalf("unsupported or ambiguous OS admitted: %q", body)
		}
	}
	for _, test := range []struct {
		value value
		want  bool
	}{
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"255.4-1ubuntu8.17"}]`)}, true},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"249.11-0ubuntu3.23"}]`)}, true},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"249.11"}]`)}, true},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"256.4"}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"248.1"}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"2550.4"}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"2490.4"}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`[{"type":"u","data":255}]`)}, false},
		{value{Type: "v", Data: json.RawMessage(`null`)}, false},
	} {
		if managerVersionValid(test.value) != test.want {
			t.Fatalf("manager version admission wrong for %s", test.value.Data)
		}
	}
}
