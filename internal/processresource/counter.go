package processresource

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func nativeCounter(raw, name string) (uint64, error) {
	var value uint64
	found := false
	for line := range strings.SplitSeq(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			if found || len(fields) != 2 {
				return 0, errors.New("native counter ambiguous")
			}
			var err error
			value, err = strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			found = true
		}
	}
	if !found {
		return 0, fmt.Errorf("native counter %s unavailable", name)
	}
	return value, nil
}

// memoryEvents holds the five memory.events.local counters the process guard
// requires. nativeMemoryEvents extracts them in one pass (audit SD-6) instead of
// re-scanning the file per counter. It preserves nativeCounter's per-name
// ambiguity, malformed-value and missing refusals, and the observe caller's
// refusal wrapping: the high counter refuses directly, while each emergency
// counter (max/oom/oom_kill/oom_group_kill) refuses under "native memory event
// invalid", in the same name order as the prior per-counter sequence.
type memoryEvents struct {
	high, max, oom, oomKill, oomGroupKill uint64
}

func nativeMemoryEvents(raw string) (memoryEvents, error) {
	var events memoryEvents
	var found [5]bool
	var refused [5]error
	destinations := [5]*uint64{&events.high, &events.max, &events.oom, &events.oomKill, &events.oomGroupKill}
	names := [5]string{"high", "max", "oom", "oom_kill", "oom_group_kill"}
	for line := range strings.SplitSeq(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		for index, name := range names {
			if fields[0] != name || refused[index] != nil {
				continue
			}
			if found[index] || len(fields) != 2 {
				refused[index] = errors.New("native counter ambiguous")
				continue
			}
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				refused[index] = err
				continue
			}
			*destinations[index] = value
			found[index] = true
		}
	}
	for index, name := range names {
		err := refused[index]
		if err == nil && !found[index] {
			err = fmt.Errorf("native counter %s unavailable", name)
		}
		if err == nil {
			continue
		}
		if index == 0 {
			return memoryEvents{}, err
		}
		return memoryEvents{}, errors.Join(errors.New("native memory event invalid"), err)
	}
	return events, nil
}

func nativePressure(raw, kind string) (float64, error) {
	found := false
	var value float64
	for line := range strings.SplitSeq(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == kind {
			if found || len(fields) != 5 || !strings.HasPrefix(fields[1], "avg10=") {
				return 0, errors.New("native pressure ambiguous")
			}
			var err error
			value, err = strconv.ParseFloat(strings.TrimPrefix(fields[1], "avg10="), 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
				return 0, errors.New("native pressure invalid")
			}
			found = true
		}
	}
	if !found {
		return 0, errors.New("native pressure unavailable")
	}
	return value, nil
}
