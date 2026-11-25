package endpoint

import (
	"errors"
	"math/big"
	"testing"
	"testing/synctest"
	"time"
)

func TestNET32IdleReserveMatchesExactAllowance(t *testing.T) {
	for _, window := range []time.Duration{1, 86399, 86400, 86401, time.Millisecond, time.Second, 18 * time.Second, 19 * time.Second, time.Minute, 10*time.Minute - 1, 10 * time.Minute} {
		work, termination := net32IdleTraffic(window)
		exact := new(big.Int).Mul(big.NewInt(int64(window)), big.NewInt(1_000_000_000))
		exact.Quo(exact, big.NewInt(int64(24*time.Hour))).Add(exact, big.NewInt(1))
		if work.Tx != exact.Uint64() || work.Rx != exact.Uint64() || termination.Tx != 8<<20 || termination.Rx != 8<<20 {
			t.Fatalf("window %v: reserve work %+v, termination %+v, exact work %v", window, work, termination, exact)
		}
	}
}

func TestNET32IdleProjectionRequiresMonotonicAndCompleteSamples(t *testing.T) {
	for _, wallElapsed := range []time.Duration{600 * time.Second, 660 * time.Second} {
		for _, observed := range []uint64{6_000_000, 7_000_000} {
			started := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			report := net32IdleReport{Started: started, Stopped: started.Add(wallElapsed), MeasuredDuration: 600 * time.Second, Samples: 601}
			series := make([]net32IdleSample, int(report.Samples))
			for second := range series {
				at := started.Add(time.Duration(second) * time.Second)
				series[second] = net32IdleSample{at: at, hostingAt: at, boot: "same-boot", ifaces: []net32IdleInterface{{name: "eth0", index: 2}}, rss: 1 << 20}
			}
			last := &series[len(series)-1]
			last.hostingAt, last.ifaces[0].tx, last.used = report.Stopped, observed, observed
			verdict := evaluateNet32Idle(report, series, 4096, nil)
			if want := observed * 144; verdict.Projected24HourBytes != want || net32IdleCriteriaPassed(verdict) != (observed == 6_000_000) {
				t.Fatalf("wall %v bytes %d: projection and criteria = %+v, want %d", wallElapsed, observed, verdict, want)
			}
			if net32IdleCriteriaPassed(evaluateNet32Idle(report, series[:599], 4096, nil)) {
				t.Fatal("incomplete series passed")
			}
			short := report
			short.MeasuredDuration = 599 * time.Second
			if net32IdleCriteriaPassed(evaluateNet32Idle(short, series, 4096, nil)) {
				t.Fatal("short monotonic observation passed")
			}
			last.ifaces = nil
			if net32IdleCriteriaPassed(evaluateNet32Idle(report, series, 4096, nil)) {
				t.Fatal("disappeared interface passed")
			}
			series[0].ifaces = nil
			last.used = series[0].used
			if net32IdleCriteriaPassed(evaluateNet32Idle(report, series, 4096, nil)) {
				t.Fatal("missing interface evidence passed as zero traffic")
			}
		}
	}
}

func net32IdleCriteriaPassed(verdict net32IdleVerdict) bool {
	for _, criterion := range verdict.Criteria {
		if !criterion.Passed {
			return false
		}
	}
	return len(verdict.Criteria) > 0
}

func TestNET32IdleBracketsFreshReads(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		clock := []time.Time{started, started.Add(2 * time.Second), started.Add(602 * time.Second), started.Add(606 * time.Second)}
		wall := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
		reads, ticks := 0, 0
		_, report, err := collectNet32Idle(t.Context(), time.Millisecond, func() time.Time {
			instant := clock[ticks]
			ticks++
			return instant
		}, func(fresh bool) (net32IdleSample, error) {
			if !fresh {
				t.Fatal("counter boundaries must request a fresh reading")
			}
			defer func() { reads++ }()
			return net32IdleSample{hostingAt: wall.Add(time.Duration(reads) * 660 * time.Second)}, nil
		})
		if err != nil || report.MeasuredDuration != 600*time.Second || report.Samples != 2 || !report.Started.Equal(wall) || !report.Stopped.Equal(wall.Add(660*time.Second)) {
			t.Fatalf("fresh monotonic boundary: %+v / %v", report, err)
		}
	})
}

func TestNET32IdleRetainsSamplingFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		failure := errors.New("counter continuity unavailable")
		reads := 0
		_, report, err := collectNet32Idle(t.Context(), time.Millisecond, time.Now, func(bool) (net32IdleSample, error) {
			reads++
			if reads == 2 {
				return net32IdleSample{}, failure
			}
			return net32IdleSample{}, nil
		})
		if !errors.Is(err, failure) || report.MeasuredDuration != 0 {
			t.Fatalf("failed sample produced an interval: %+v / %v", report, err)
		}
	})
}
