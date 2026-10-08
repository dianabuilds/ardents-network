package cgroup

import (
	"strconv"
	"strings"
)

// This kernel observation is not invocation or stop authority. Missing,
// duplicate and unknown observations cannot report physical completion.
func parseEvents(body []byte) (bool, error) {
	if len(body) == 0 || len(body) > 1024 {
		return false, errBinding
	}
	fields := strings.Fields(string(body))
	if len(fields) != 4 {
		return false, errBinding
	}
	values := make(map[string]string, 2)
	for index := 0; index < len(fields); index += 2 {
		key, value := fields[index], fields[index+1]
		if (key != "populated" && key != "frozen") || (value != "0" && value != "1") || values[key] != "" {
			return false, errBinding
		}
		values[key] = value
	}
	return values["populated"] == "1", nil
}

// A name restricts an inventory; it never proves a process or invocation.
func workerScope(unit string) (pid, uid uint32, valid bool) {
	for _, role := range []string{"reader", "publisher"} {
		prefix := "ardents-text-" + role + "@"
		if !strings.HasPrefix(unit, prefix) || !strings.HasSuffix(unit, ".service") {
			continue
		}
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(unit, prefix), ".service"), "-")
		if len(parts) != 3 {
			return 0, 0, false
		}
		var values [3]uint32
		for index, part := range parts {
			value, err := strconv.ParseUint(part, 10, 32)
			if err != nil || strconv.FormatUint(value, 10) != part {
				return 0, 0, false
			}
			values[index] = uint32(value)
		}
		return values[1], values[2], values[1] != 0 && values[2] != 0
	}
	return 0, 0, false
}

func scopeUnit(unit string) bool {
	if unit == "ardents-endpoint.service" {
		return true
	}
	_, _, valid := workerScope(unit)
	return valid
}

func workerMatches(unit string, pid, uid uint32) bool {
	actualPID, actualUID, valid := workerScope(unit)
	return valid && actualPID == pid && actualUID == uid
}
