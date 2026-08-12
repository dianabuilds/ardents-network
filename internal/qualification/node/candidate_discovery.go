package node

import (
	"errors"
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
