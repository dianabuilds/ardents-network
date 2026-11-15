package processresource

import (
	"errors"
	"strconv"
	"strings"
)

// ParseStartClock validates detached Linux proc-stat bytes for the exact PID.
// A parsed value is not evidence that the process or its cgroup is still live.
func ParseStartClock(body []byte, pid uint32) (uint64, error) {
	text := string(body)
	begin, end := strings.Index(text, " ("), strings.LastIndex(text, ") ")
	if begin < 1 || end <= begin || text[:begin] != strconv.FormatUint(uint64(pid), 10) {
		return 0, errors.New("invalid process identity")
	}
	fields := strings.Fields(text[end+2:])
	if len(fields) < 20 || len(fields[0]) != 1 || !strings.Contains("RSDITt", fields[0]) {
		return 0, errors.New("invalid process identity")
	}
	started, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || started == 0 || strconv.FormatUint(started, 10) != fields[19] {
		return 0, errors.New("invalid process identity")
	}
	return started, nil
}
