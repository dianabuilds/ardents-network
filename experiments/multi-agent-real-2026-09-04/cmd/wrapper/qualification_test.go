//go:build ignore

package main

import (
	"bytes"
	"reflect"
	"testing"
	"time"
)

func TestRecordedActionsUseStableCommandsAndVerify(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	base := time.Date(2026, 9, 4, 2, 0, 0, 0, time.UTC)
	_, manifest, err := prepareRun(evidence, clockAt(base), bytes.NewReader(bytes.Repeat([]byte{0x41}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	honestRunner := &capturingRunner{output: testSourceOutput([4]string{"valid", "valid", "not-attempted", "not-attempted"})}
	refresh := actionRequest{Schema: actionSchema, Action: "refresh"}
	for index := 0; index < 2; index++ {
		if _, _, err := recordAction(manifest, "honest_user", refresh, honestRunner, clockAt(base.Add(time.Duration(index)*35*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	if len(honestRunner.calls) != 2 || !reflect.DeepEqual(honestRunner.calls[0], honestRunner.calls[1]) {
		t.Fatalf("honest commands drifted: %q", honestRunner.calls)
	}
	batteryRunner := &capturingRunner{output: honestRunner.output}
	if _, _, err := recordAction(manifest, "battery_saver", refresh, batteryRunner, clockAt(base)); err != nil {
		t.Fatal(err)
	}
	noop := actionRequest{Schema: actionSchema, Action: "noop", Reason: "accepted generation unchanged"}
	if _, _, err := recordAction(manifest, "battery_saver", noop, batteryRunner, clockAt(base.Add(150*time.Second))); err != nil {
		t.Fatal(err)
	}
	probeRunner := &capturingRunner{output: testSourceOutput([4]string{"valid", "invalid-state", "not-attempted", "not-attempted"})}
	if _, event, err := recordAction(manifest, "probe_consumer", refresh, probeRunner, clockAt(base)); err != nil {
		t.Fatal(err)
	} else if event.Kind != "reject" {
		t.Fatalf("probe kind = %q", event.Kind)
	}
	if _, err := verifyRun(manifest); err == nil {
		t.Fatal("verifier accepted evidence below the contract minimums")
	}
	for index := 2; index < 10; index++ {
		if _, _, err := recordAction(manifest, "honest_user", refresh, honestRunner, clockAt(base.Add(time.Duration(index)*35*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := recordAction(manifest, "battery_saver", noop, batteryRunner, clockAt(base.Add(300*time.Second))); err != nil {
		t.Fatal(err)
	}
	for index := 1; index < 5; index++ {
		if _, _, err := recordAction(manifest, "probe_consumer", noop, probeRunner, clockAt(base.Add(time.Duration(index)*45*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := verifyRun(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(summary.Events, map[string]int{"honest_user": 10, "battery_saver": 3, "probe_consumer": 5}) {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.Generation != "generation-a" {
		t.Fatalf("verified generation = %q", summary.Generation)
	}
}
