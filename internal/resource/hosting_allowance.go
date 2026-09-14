package resource

import (
	"errors"
	"math"
	"time"
)

// HostingPolicy is an operator-supplied billing period. Quantity and Unit retain
// the provider's original denomination; InitialUsedBytes is its consumed floor
// at initialization, not a new allowance for each Ardents process.
type HostingPolicy struct {
	Provider          string    `json:"provider"`
	Start             time.Time `json:"start"`
	End               time.Time `json:"end"`
	Unit              string    `json:"unit"`
	Quantity          uint64    `json:"quantity"`
	Directions        string    `json:"directions"`
	Interfaces        []string  `json:"interfaces"`
	InitialUsedBytes  uint64    `json:"initial_used_bytes"`
	LowWatermarkBytes uint64    `json:"low_watermark_bytes"`
}

// HostingTraffic keeps ingress and egress separate until the actual provider's
// counted directions are applied. It describes an upper reservation, not a
// measurement of useful Application bytes or an admission token's authority.
type HostingTraffic struct {
	Tx uint64 `json:"tx"`
	Rx uint64 `json:"rx"`
}

// HostingObservation reports a shared host's durable period accounting.
// Resource decides pressure; the consuming Node or Endpoint owns shutdown.
type HostingObservation struct {
	UsedBytes      uint64
	ReservedBytes  uint64
	RemainingBytes uint64
	Protect        bool
	Drain          bool
}

func (policy HostingPolicy) limit() (uint64, error) {
	units := map[string]uint64{"B": 1, "MB": 1000000, "MiB": 1 << 20, "GB": 1000000000, "GiB": 1 << 30, "TB": 1000000000000, "TiB": 1 << 40}
	unit, found := units[policy.Unit]
	if !found || policy.Quantity == 0 || policy.Quantity > math.MaxUint64/unit {
		return 0, errors.New("hosting allowance denomination is invalid")
	}
	limit := policy.Quantity * unit
	if len(policy.Provider) == 0 || len(policy.Provider) > 128 || policy.Start.IsZero() || !policy.Start.Before(policy.End) ||
		policy.Start.Nanosecond() != 0 || policy.End.Nanosecond() != 0 || policy.InitialUsedBytes > limit ||
		policy.LowWatermarkBytes == 0 || policy.LowWatermarkBytes >= limit || len(policy.Interfaces) == 0 || len(policy.Interfaces) > 16 {
		return 0, errors.New("hosting allowance period or bound is invalid")
	}
	if policy.Directions != "tx" && policy.Directions != "rx" && policy.Directions != "tx+rx" {
		return 0, errors.New("hosting counted directions are invalid")
	}
	seen := make(map[string]bool, len(policy.Interfaces))
	for _, name := range policy.Interfaces {
		if name == "" || len(name) > 15 || seen[name] || name == "." || name == ".." {
			return 0, errors.New("hosting interface inventory is invalid")
		}
		for _, char := range name {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' || char == '.') {
				return 0, errors.New("hosting interface name is invalid")
			}
		}
		seen[name] = true
	}
	return limit, nil
}

func (policy HostingPolicy) cost(traffic HostingTraffic) (uint64, error) {
	switch policy.Directions {
	case "tx":
		return traffic.Tx, nil
	case "rx":
		return traffic.Rx, nil
	case "tx+rx":
		if traffic.Tx <= math.MaxUint64-traffic.Rx {
			return traffic.Tx + traffic.Rx, nil
		}
	}
	return 0, errors.New("hosting traffic accounting is unavailable")
}

type hostingInterface struct {
	Name  string `json:"name"`
	Index uint64 `json:"index"`
	Tx    uint64 `json:"tx"`
	Rx    uint64 `json:"rx"`
}

type hostingReading struct {
	Boot       string             `json:"boot"`
	Interfaces []hostingInterface `json:"interfaces"`
}

// The committed snapshot keeps pending reservations after process death. A new
// process has no reservation handle with which to refund an ambiguous old job.
type hostingState struct {
	Schema   string         `json:"schema"`
	Policy   HostingPolicy  `json:"policy"`
	Used     uint64         `json:"used"`
	Reserved uint64         `json:"reserved"`
	Observed time.Time      `json:"observed"`
	Reading  hostingReading `json:"reading"`
}

func (state *hostingState) observe(reading hostingReading, now time.Time) error {
	if now.Before(state.Observed) || reading.Boot == "" || reading.Boot != state.Reading.Boot || len(reading.Interfaces) != len(state.Reading.Interfaces) {
		return errors.New("hosting observation continuity is unavailable")
	}
	var total HostingTraffic
	for index, prior := range state.Reading.Interfaces {
		current := reading.Interfaces[index]
		if current.Name != prior.Name || current.Index != prior.Index || current.Tx < prior.Tx || current.Rx < prior.Rx ||
			current.Tx-prior.Tx > math.MaxUint64-total.Tx || current.Rx-prior.Rx > math.MaxUint64-total.Rx {
			return errors.New("hosting interface counters lost continuity")
		}
		total.Tx += current.Tx - prior.Tx
		total.Rx += current.Rx - prior.Rx
	}
	cost, err := state.Policy.cost(total)
	if err != nil || cost > math.MaxUint64-state.Used {
		return errors.New("hosting period counter overflow")
	}
	state.Used += cost
	state.Reading, state.Observed = reading, now
	return nil
}

func (state hostingState) observation(now time.Time) HostingObservation {
	limit, err := state.Policy.limit()
	result := HostingObservation{UsedBytes: state.Used, ReservedBytes: state.Reserved, Protect: true, Drain: true}
	if err != nil || now.Before(state.Policy.Start) || !now.Before(state.Policy.End) || state.Used > limit || state.Reserved > limit-state.Used {
		return result
	}
	result.RemainingBytes = limit - state.Used - state.Reserved
	result.Protect = result.RemainingBytes <= state.Policy.LowWatermarkBytes
	// Reserved work remains owned. A low watermark prevents new reservations;
	// actual consumption reaching the unreserved margin requires bounded drain.
	result.Drain = result.RemainingBytes == 0
	return result
}
