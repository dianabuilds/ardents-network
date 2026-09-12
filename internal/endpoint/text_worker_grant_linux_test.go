//go:build linux

package endpoint

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

func TestCompletedTextWorkerResultCannotCrossReplacement(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	worker := &qualifiedTextWorker{job: job}
	if worker.completedCurrent() {
		t.Fatal("unjoined job supplied a result")
	}
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	if !worker.completedCurrent() {
		t.Fatal("joined current result refused")
	}
	replacement, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	if worker.completedCurrent() {
		t.Fatal("old result entered a live replacement")
	}
	owner.retireJob(replacement)
	if err := owner.finishJobCleanup(replacement, nil); err != nil {
		t.Fatal(err)
	}
	if worker.completedCurrent() {
		t.Fatal("old result resurrected after replacement ended")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if (&qualifiedTextWorker{job: replacement}).completedCurrent() {
		t.Fatal("result survived context revoke")
	}
}

func TestTextWorkerOperationCannotReserveTwiceOrAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	lifetime := &textWorkerLifetime{context: ctx}
	finish, err := lifetime.beginUse()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lifetime.beginUse(); err == nil {
		t.Fatal("worker operation was consumed twice")
	}
	finish()
	cancel()
	if _, err := lifetime.beginUse(); err == nil {
		t.Fatal("finished operation was reused")
	}
	lifetime = &textWorkerLifetime{context: ctx}
	if _, err := lifetime.beginUse(); err == nil {
		t.Fatal("cancelled lifetime admitted work")
	}
}

func TestAlreadyCancelledTextLaunchHasNoEffects(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	release, err := endpoint.acquireTextLaunch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if worker, err := owner.launchTextWorker(ctx, nil); err == nil || worker != nil {
		t.Fatal("cancelled launch accepted")
	}
	owner.mu.Lock()
	pending := owner.job != nil
	owner.mu.Unlock()
	if pending {
		t.Fatal("cancelled launch retained a job")
	}
	if !endpoint.textAvailable() {
		t.Fatal("no-effect cancellation terminalized Endpoint")
	}
}

func TestEndpointMainExitCannotLeaveItsUnitLogicallyActive(t *testing.T) {
	valid := textManagerProperties{
		"RemainAfterExit": {Type: "b", Data: json.RawMessage("false")},
		"ExitType":        {Type: "s", Data: json.RawMessage(`"main"`)},
		"RestartMode":     {Type: "s", Data: json.RawMessage(`"normal"`)},
	}
	if !textEndpointStopsWithMain(valid) {
		t.Fatal("selected Endpoint lifetime refused")
	}
	for _, test := range []struct {
		name  string
		value textManagerValue
	}{
		{"RemainAfterExit", textManagerValue{Type: "b", Data: json.RawMessage("true")}},
		{"RemainAfterExit", textManagerValue{Type: "b", Data: json.RawMessage("null")}},
		{"ExitType", textManagerValue{Type: "s", Data: json.RawMessage(`"cgroup"`)}},
		{"RestartMode", textManagerValue{Type: "s", Data: json.RawMessage(`"direct"`)}},
		{"RestartMode", textManagerValue{}},
	} {
		previous := valid[test.name]
		valid[test.name] = test.value
		if textEndpointStopsWithMain(valid) {
			t.Errorf("accepted parent lifetime %s=%s", test.name, test.value.Data)
		}
		valid[test.name] = previous
	}
}

func TestTextLaunchCancellationReleasesWaitingReservation(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	release, err := endpoint.acquireTextLaunch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := owner.launchTextWorker(ctx, nil); finished <- err }()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		owner.mu.Lock()
		reserved := owner.job != nil
		owner.mu.Unlock()
		if reserved {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("launch did not reserve its waiting job")
		case <-ticker.C:
		}
	}
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled waiting launch accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("waiting launch did not join cancellation")
	}
	owner.mu.Lock()
	pending := owner.job != nil
	owner.mu.Unlock()
	if pending || !endpoint.textAvailable() {
		t.Fatal("no-effect waiting cancellation retained pressure or terminalized Endpoint")
	}
}

func TestTextLaunchRejectsOtherOrUnknownPlatform(t *testing.T) {
	if !textUbuntuRelease([]byte("ID=ubuntu\nVERSION_ID=\"24.04\"\n")) {
		t.Fatal("selected OS identity refused")
	}
	for _, body := range []string{"ID=debian\nVERSION_ID=24.04\n", "ID=ubuntu\nVERSION_ID=26.04\n", "ID=ubuntu\n", "ID=ubuntu\nID=debian\nVERSION_ID=24.04\n"} {
		if textUbuntuRelease([]byte(body)) {
			t.Fatal("unsupported or ambiguous OS admitted")
		}
	}
	for _, test := range []struct {
		value textManagerValue
		want  bool
	}{
		{textManagerValue{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"255.4-1ubuntu8.17"}]`)}, true},
		{textManagerValue{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"256.4"}]`)}, false},
		{textManagerValue{Type: "v", Data: json.RawMessage(`[{"type":"s","data":"2550.4"}]`)}, false},
		{textManagerValue{Type: "v", Data: json.RawMessage(`[{"type":"u","data":255}]`)}, false},
		{textManagerValue{Type: "v", Data: json.RawMessage(`null`)}, false},
	} {
		if textSystemdVersion(test.value) != test.want {
			t.Fatal("unknown system manager version admitted or selected version refused")
		}
	}
}
