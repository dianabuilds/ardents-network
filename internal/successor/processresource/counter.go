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
	for _, line := range strings.Split(raw, "\n") {
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
func nativePressure(raw, kind string) (float64, error) {
	found := false
	var value float64
	for _, line := range strings.Split(raw, "\n") {
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
