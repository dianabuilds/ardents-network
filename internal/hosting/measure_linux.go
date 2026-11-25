//go:build linux

package hosting

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
)

func hostingPlatform() error { return nil }

func measureHosting(names []string) (hostingReading, error) {
	var reading hostingReading
	boot, err := boundedFile("/proc/sys/kernel/random/boot_id", 64)
	if err != nil {
		return reading, err
	}
	reading.Boot = strings.TrimSpace(string(boot))
	if len(reading.Boot) != 36 {
		return reading, errors.New("hosting boot identity is unavailable")
	}
	for _, name := range names {
		directory := filepath.Join("/sys/class/net", name)
		values := make([]uint64, 3)
		for index, field := range []string{"ifindex", "statistics/tx_bytes", "statistics/rx_bytes"} {
			raw, err := boundedFile(filepath.Join(directory, field), 64)
			if err != nil {
				return hostingReading{}, err
			}
			values[index], err = strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
			if err != nil {
				return hostingReading{}, errors.New("hosting interface counter is unavailable")
			}
		}
		// Recheck identity after sampling: a removed/recreated device is no longer
		// the counter source bound to this period, even when its name is reused.
		after, err := boundedFile(filepath.Join(directory, "ifindex"), 64)
		if err != nil || strings.TrimSpace(string(after)) != strconv.FormatUint(values[0], 10) {
			return hostingReading{}, errors.New("hosting interface changed during observation")
		}
		reading.Interfaces = append(reading.Interfaces, hostingInterface{Name: name, Index: values[0], Tx: values[1], Rx: values[2]})
	}
	return reading, nil
}
