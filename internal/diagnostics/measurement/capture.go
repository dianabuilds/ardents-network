package measurement

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

// Only an explicitly started command capture activates producers. No init,
// environment read, implicit exporter, or domain object is retained here.
var selected atomic.Pointer[Capture]

type Capture struct {
	mu        sync.Mutex
	started   time.Time
	stopped   bool
	values    map[runtimeevent.Instrument]runtimeevent.Measurement
	history   []runtimeevent.MeasurementSnapshot
	discarded uint64
}

// Start selects one finite local capture. Concurrent/nested capture is refused.
func Start() (*Capture, error) {
	c := &Capture{started: time.Now().UTC(), values: make(map[runtimeevent.Instrument]runtimeevent.Measurement)}
	if !selected.CompareAndSwap(nil, c) {
		return nil, errors.New("measurement capture already active")
	}
	return c, nil
}

func (c *Capture) Stop() {
	selected.CompareAndSwap(c, nil)
	c.mu.Lock()
	if !c.stopped {
		c.stopped = true
		c.sampleLocked()
	}
	c.mu.Unlock()
}

func (c *Capture) Sample() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		c.sampleLocked()
	}
}
func (c *Capture) sampleLocked() {
	point := runtimeevent.MeasurementSnapshot{Schema: runtimeevent.MeasurementSchema, Delivery: runtimeevent.ExportLocalOnly, At: time.Now().UTC(), Started: c.started, Values: []runtimeevent.Measurement{}}
	for _, id := range runtimeevent.Instruments() {
		if value, ok := c.values[id]; ok {
			point.Values = append(point.Values, value)
		}
	}
	if len(c.history) == runtimeevent.MeasurementHistoryLimit {
		copy(c.history, c.history[1:])
		c.history = c.history[:runtimeevent.MeasurementHistoryLimit-1]
		c.discarded++
	}
	point.Discarded = c.discarded
	c.history = append(c.history, point)
}

// History returns detached samples after Stop; it never calls a domain owner.
func (c *Capture) History() []runtimeevent.MeasurementSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := append([]runtimeevent.MeasurementSnapshot(nil), c.history...)
	for i := range out {
		out[i].Values = append([]runtimeevent.Measurement{}, out[i].Values...)
	}
	return out
}

// Gauge is bound to the capture active when the original owner is constructed.
// Late releases cannot subtract from a later capture or another owner's state.
type Gauge struct {
	capture *Capture
	id      runtimeevent.Instrument
}

func Track(id runtimeevent.Instrument) Gauge {
	c := selected.Load()
	spec, ok := id.Spec()
	if c == nil || !ok || spec.Kind != runtimeevent.GaugeInstrument {
		return Gauge{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return Gauge{}
	}
	if _, ok := c.values[id]; !ok {
		c.values[id] = runtimeevent.Measurement{Name: spec.Name, Available: true}
	}
	return Gauge{c, id}
}
func (g Gauge) Add(delta int64) {
	if delta < 0 {
		g.Decrease(uint64(-(delta + 1)) + 1)
	} else {
		g.Increase(uint64(delta))
	}
}

// Stage records elapsed monotonic time and active calls. End is idempotent.
// Disabled recording does not read the clock, allocate or access domain state.
type Stage struct {
	capture *Capture
	id      runtimeevent.Instrument
	at      time.Time
	ended   atomic.Bool
}

func Begin(id runtimeevent.Instrument) *Stage {
	c := selected.Load()
	spec, ok := id.Spec()
	if c == nil || !ok || spec.Kind != runtimeevent.StageInstrument {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return nil
	}
	v, exists := c.values[id]
	if !exists {
		v = runtimeevent.Measurement{Name: spec.Name, Available: true}
	}
	if v.Current == runtimeevent.MaximumMeasurementInteger {
		v.Available = false
	} else {
		v.Current++
		v.Peak = max(v.Peak, v.Current)
	}
	c.values[id] = v
	return &Stage{capture: c, id: id, at: time.Now()}
}
func (s *Stage) End() {
	if s == nil || !s.ended.CompareAndSwap(false, true) {
		return
	}
	elapsed := time.Since(s.at).Seconds()
	c := s.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	v := c.values[s.id]
	if !v.Available {
		return
	}
	if v.Current == 0 || v.Count == runtimeevent.MaximumMeasurementInteger || math.IsInf(v.Seconds+elapsed, 0) {
		v.Available = false
	} else {
		v.Current--
		v.Count++
		v.Seconds += elapsed
		v.MaximumSeconds = max(v.MaximumSeconds, elapsed)
	}
	c.values[s.id] = v
}

// Latest returns the last detached sample without querying any domain owner.
func (c *Capture) Latest() runtimeevent.MeasurementSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.history) == 0 {
		return runtimeevent.MeasurementSnapshot{}
	}
	point := c.history[len(c.history)-1]
	point.Values = append([]runtimeevent.Measurement{}, point.Values...)
	return point
}

// Increase and Decrease preserve unsigned owner bounds without int conversion.
func (g Gauge) Increase(amount uint64) { g.change(amount, false) }
func (g Gauge) Decrease(amount uint64) { g.change(amount, true) }
func (g Gauge) change(amount uint64, subtract bool) {
	if g.capture == nil {
		return
	}
	c := g.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	v := c.values[g.id]
	if !v.Available {
		return
	}
	if amount > runtimeevent.MaximumMeasurementInteger || subtract && amount > v.Current || !subtract && amount > runtimeevent.MaximumMeasurementInteger-v.Current {
		v.Available = false
	} else if subtract {
		v.Current -= amount
	} else {
		v.Current += amount
		v.Peak = max(v.Peak, v.Current)
	}
	c.values[g.id] = v
}

// ObserveLast records a confirmed quantity from the last owning transaction.
// These sample instruments are not summed across roots/handles. Unavailable
// invalidates the last observation rather than substituting measured zero.
func ObserveLast(id runtimeevent.Instrument, value uint64) { observeLast(id, value, true) }
func Unavailable(id runtimeevent.Instrument)               { observeLast(id, 0, false) }
func observeLast(id runtimeevent.Instrument, quantity uint64, available bool) {
	c := selected.Load()
	spec, ok := id.Spec()
	if c == nil || !ok || spec.Kind != runtimeevent.LastSampleInstrument {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	v := c.values[id]
	v.Name = spec.Name
	v.Available = available && quantity <= runtimeevent.MaximumMeasurementInteger
	if v.Available {
		v.Current = quantity
		v.Peak = max(v.Peak, quantity)
	}
	c.values[id] = v
}
