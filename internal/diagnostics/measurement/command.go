package measurement

import (
	"encoding/json"
	"io"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

// Exporter is the capture consumer's bounded optional delivery owner. Domain
// producers never invoke it; telemetry owns its request and close deadlines.
type Exporter interface {
	ExportMeasurements(runtimeevent.MeasurementSnapshot) bool
	Close() bool
}

// Run records one explicit local command capture. Sampling retains at most 32
// aggregate snapshots in memory, with overwrite accounting. Producers never
// wait for output or a collector. Emission happens after original work returns;
// no domain resource or cleanup is transferred to this diagnostic lifetime.
// Caller owns the private local sink. A sink failure changes only capture exit
// when the original command succeeded; original failures are preserved.
// Run consumes the optional Exporter, including on setup refusal.
func Run(output io.Writer, work func() int, export Exporter) int {
	if output == nil || work == nil {
		if export != nil {
			_ = export.Close()
		}
		return 2
	}
	capture, err := Start()
	if err != nil {
		if export != nil {
			_ = export.Close()
		}
		return 2
	}
	var failed atomic.Bool
	stop, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				capture.Sample()
				if export != nil && !export.ExportMeasurements(capture.Latest()) {
					failed.Store(true)
				}
			}
		}
	}()
	code := work()
	close(stop)
	<-joined
	capture.Stop()
	if export != nil && !export.ExportMeasurements(capture.Latest()) {
		failed.Store(true)
	}
	if export != nil && !export.Close() {
		failed.Store(true)
	}
	if _, err = io.WriteString(output, "\n"); err == nil {
		encoder := json.NewEncoder(output)
		for _, point := range capture.History() {
			if export != nil {
				point.Delivery = runtimeevent.ExportCompleted
				if failed.Load() {
					point.Delivery = runtimeevent.ExportUnavailable
				}
			}
			if err = encoder.Encode(point); err != nil {
				break
			}
		}
	}
	if err != nil && code == 0 {
		return 1
	}
	return code
}
