package measurement

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func startCapture(t *testing.T) *Capture {
	t.Helper()
	c, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	return c
}
func measured(t *testing.T, c *Capture, id runtimeevent.Instrument) runtimeevent.Measurement {
	t.Helper()
	c.Sample()
	spec, _ := id.Spec()
	for _, v := range c.Latest().Values {
		if v.Name == spec.Name {
			return v
		}
	}
	t.Fatalf("instrument %s not observed", spec.Name)
	return runtimeevent.Measurement{}
}

func TestStageAndResourceRetainActualLifetimeAndPeak(t *testing.T) {
	c := startCapture(t)
	stage := Begin(runtimeevent.ExecutionLaunch)
	gauge := Track(runtimeevent.ExecutionCleanupSlots)
	gauge.Add(1)
	active := measured(t, c, runtimeevent.ExecutionLaunch)
	if active.Current != 1 || active.Count != 0 || active.Seconds != 0 {
		t.Fatal("unfinished call fabricated duration", active)
	}
	// Preserve the monotonic component and supply a known elapsed interval.
	// An immediate Begin/End may legitimately share one Windows clock tick.
	stage.at = stage.at.Add(-time.Second)
	stage.End()
	stage.End()
	ended := measured(t, c, runtimeevent.ExecutionLaunch)
	held := measured(t, c, runtimeevent.ExecutionCleanupSlots)
	if ended.Current != 0 || ended.Count != 1 || ended.Seconds < time.Second.Seconds() || ended.MaximumSeconds != ended.Seconds || held.Current != 1 {
		t.Fatal("call completion returned physical cleanup capacity", ended, held)
	}
	gauge.Add(-1)
	if retired := measured(t, c, runtimeevent.ExecutionCleanupSlots); retired.Current != 0 || retired.Peak != 1 || !retired.Available {
		t.Fatal(retired)
	}
}
func TestLateReleaseCannotChangeNextCapture(t *testing.T) {
	c := startCapture(t)
	old := Track(runtimeevent.RouteReservedBytes)
	old.Increase(7)
	c.Stop()
	next := startCapture(t)
	current := Track(runtimeevent.RouteReservedBytes)
	current.Increase(3)
	old.Decrease(7)
	if got := measured(t, next, runtimeevent.RouteReservedBytes); got.Current != 3 {
		t.Fatal("foreign release crossed capture", got)
	}
	if got := c.Latest().Values[0]; got.Current != 7 {
		t.Fatal("stopped evidence mutated", got)
	}
}
func TestInvalidBalancesBecomeUnavailableWithoutUnsignedWrap(t *testing.T) {
	c := startCapture(t)
	g := Track(runtimeevent.RouteReservedBytes)
	g.Decrease(1)
	if got := measured(t, c, runtimeevent.RouteReservedBytes); got.Available {
		t.Fatal("underflow became valid zero", got)
	}
	h := Track(runtimeevent.RouteChildren)
	h.Increase(^uint64(0))
	if got := measured(t, c, runtimeevent.RouteChildren); got.Available {
		t.Fatal("overflow became valid quantity", got)
	}
}
func TestConcurrentOwnersAggregateAndJoinWithoutLoss(t *testing.T) {
	c := startCapture(t)
	var users sync.WaitGroup
	for i := 0; i < 40; i++ {
		users.Go(func() {
			g := Track(runtimeevent.RouteReservedBytes)
			g.Increase(8)
			defer g.Decrease(8)
			defer Begin(runtimeevent.ConnectionRead).End()
		})
	}
	users.Wait()
	if got := measured(t, c, runtimeevent.RouteReservedBytes); !got.Available || got.Current != 0 || got.Peak < 8 {
		t.Fatal(got)
	}
	if got := measured(t, c, runtimeevent.ConnectionRead); got.Current != 0 || got.Count != 40 {
		t.Fatal(got)
	}
}
func TestHistoryIsBoundedDetachedAndReportsOverwrites(t *testing.T) {
	c := startCapture(t)
	g := Track(runtimeevent.RouteChildren)
	g.Add(1)
	for i := 0; i < 40; i++ {
		c.Sample()
	}
	c.Stop()
	c.Stop()
	history := c.History()
	if len(history) != runtimeevent.MeasurementHistoryLimit || history[len(history)-1].Discarded != 9 {
		t.Fatal("history accounting", len(history), history[len(history)-1].Discarded)
	}
	history[0].Values[0].Current = 99
	if c.History()[0].Values[0].Current != 1 {
		t.Fatal("borrowed mutable evidence")
	}
}
func TestDisabledRecordingAndEmptyCaptureHaveDistinctEvidence(t *testing.T) {
	if Begin(runtimeevent.CustodyDecrypt) != nil {
		t.Fatal("implicit recording")
	}
	var out bytes.Buffer
	if code := Run(&out, func() int { return 2 }, nil); code != 2 {
		t.Fatal("capture erased refusal")
	}
	record, present, err := runtimeevent.Decode(bytes.TrimSpace(out.Bytes()))
	if err != nil || !present || record.Measurements == nil || len(record.Measurements.Values) != 0 {
		t.Fatal("empty capture fabricated measured zero", present, err, out.String())
	}
}

type refusedExporter struct{ closed bool }

func (*refusedExporter) ExportMeasurements(runtimeevent.MeasurementSnapshot) bool { return false }
func (e *refusedExporter) Close() bool                                            { e.closed = true; return false }
func TestExporterFailureKeepsCommandAndLocalMeasurements(t *testing.T) {
	var out bytes.Buffer
	export := new(refusedExporter)
	code := Run(&out, func() int { defer Begin(runtimeevent.EnrollmentVerify).End(); return 0 }, export)
	record, present, err := runtimeevent.Decode(bytes.TrimSpace(out.Bytes()))
	if code != 0 || !export.closed || err != nil || !present || record.Measurements.Delivery != runtimeevent.ExportUnavailable || len(record.Measurements.Values) != 1 {
		t.Fatal(code, err, out.String())
	}
}

type brokenSink struct{}

func (brokenSink) Write([]byte) (int, error) { return 0, errors.New("private-sentinel") }
func TestSinkFailurePreservesOriginalFailureAndNeverEchoesError(t *testing.T) {
	if code := Run(brokenSink{}, func() int { return 130 }, nil); code != 130 {
		t.Fatal(code)
	}
	var out bytes.Buffer
	if code := Run(&out, func() int { return 0 }, nil); code != 0 || strings.Contains(out.String(), "private-sentinel") {
		t.Fatal(code)
	}
}

type heldExporter struct {
	entered, release chan struct{}
	once             sync.Once
}

func (e *heldExporter) ExportMeasurements(runtimeevent.MeasurementSnapshot) bool {
	e.once.Do(func() { close(e.entered); <-e.release })
	return false
}
func (e *heldExporter) Close() bool { return true }
func TestBlockedExporterCannotBlockOwnerTransitions(t *testing.T) {
	exporter := &heldExporter{entered: make(chan struct{}), release: make(chan struct{})}
	var out bytes.Buffer
	producer := make(chan struct{})
	done := make(chan int, 1)
	go func() {
		done <- Run(&out, func() int {
			<-exporter.entered
			g := Track(runtimeevent.RouteChildren)
			g.Add(1)
			g.Add(-1)
			Begin(runtimeevent.NetworkRefresh).End()
			close(producer)
			return 0
		}, exporter)
	}()
	select {
	case <-producer:
	case <-time.After(5 * time.Second):
		close(exporter.release)
		t.Fatal("export blocked producer")
	}
	close(exporter.release)
	select {
	case code := <-done:
		if code != 0 {
			t.Fatal(code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("capture did not join")
	}
	if !bytes.Contains(out.Bytes(), []byte(`"telemetry":"unavailable"`)) {
		t.Fatal("export failure hidden")
	}
}
