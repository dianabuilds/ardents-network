package node

import (
	"errors"
	"sort"
	"strconv"
	"strings"
)

func nodeCandidateFromInspect(service, shortID, raw string) (nodeHostCandidate, bool, error) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[0], shortID) {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid < 0 {
			return nodeHostCandidate{}, false, errors.New("node resource candidate host PID is invalid")
		}
		if pid == 0 {
			return nodeHostCandidate{}, false, nil
		}
		return nodeHostCandidate{Service: service, ContainerID: fields[0], PID: pid}, true, nil
	}
	return nodeHostCandidate{}, false, nil
}

func nearestRank95(values []uint64) uint64 {
	ordered := append([]uint64(nil), values...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	index := (95*len(ordered)+99)/100 - 1
	return ordered[index]
}

func nodeRawCounter(raw, name string) (uint64, bool) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == name {
			value, err := strconv.ParseUint(fields[1], 10, 64)
			return value, err == nil
		}
	}
	return 0, false
}
