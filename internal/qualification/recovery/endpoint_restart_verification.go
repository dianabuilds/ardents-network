package recovery

import (
	"encoding/hex"
	"strings"
	"time"
)

func verifyEndpointRestart(value Negative) Result {
	if value.InjectedResource != "publisher-endpoint" || !fullContainerID(value.ContainerID) {
		return invalid("Endpoint restart resource identity is incomplete")
	}
	before, beforeOK := processStartedAt(value.BeforeProcess, value.ContainerID)
	after, afterOK := processStartedAt(value.AfterProcess, value.ContainerID)
	if !beforeOK || !afterOK || !after.After(before) {
		return invalid("Endpoint restart process identity is incomplete")
	}
	return Result{Verdict: "pass"}
}

func processStartedAt(value, container string) (time.Time, bool) {
	identity, encoded, ok := strings.Cut(value, "@")
	if !ok || identity != container || strings.Contains(encoded, "@") {
		return time.Time{}, false
	}
	started, err := time.Parse(time.RFC3339Nano, encoded)
	return started, err == nil && !started.IsZero()
}

func fullContainerID(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}
