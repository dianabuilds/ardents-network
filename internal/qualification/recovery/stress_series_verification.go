package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type stressShaperConfigVerification struct {
	SchemaVersion string `json:"schema_version"`
	RunID         string `json:"run_id"`
	Role          string `json:"role"`
	Mode          string `json:"mode"`
	Profile       string `json:"profile"`
	Seed          uint32 `json:"seed"`
}

type stressShaperResultVerification struct {
	SchemaVersion         string            `json:"schema_version"`
	RunID                 string            `json:"run_id"`
	Role                  string            `json:"role"`
	Status                string            `json:"status"`
	EffectiveCapabilities string            `json:"effective_capabilities"`
	ToolLockSHA256        string            `json:"tool_lock_sha256"`
	Qdisc                 map[string]string `json:"qdisc"`
	Tools                 map[string]struct {
		Version string `json:"version"`
		Path    string `json:"path"`
		SHA256  string `json:"sha256"`
	} `json:"tools"`
}

func verifiedProgress(values []progressSample, duration int64) ([]float64, Result) {
	if duration <= 0 || duration%int64(time.Minute) != 0 || len(values) < int(duration/int64(time.Second)) {
		return nil, invalid("S4.3 host progress samples are incomplete")
	}
	if values[0].AtNanos != 0 || values[0].Delivered != 0 ||
		values[len(values)-1].AtNanos < duration || values[len(values)-1].AtNanos > duration+int64(2*time.Second) {
		return nil, invalid("S4.3 progress interval boundary is invalid")
	}
	lastAdvance := int64(0)
	for index := 1; index < len(values); index++ {
		if values[index].AtNanos <= values[index-1].AtNanos || values[index].Delivered < values[index-1].Delivered ||
			values[index].AtNanos-values[index-1].AtNanos > int64(2*time.Second) {
			return nil, invalid("S4.3 progress series is not monotonic or one-second bounded")
		}
		if values[index].Delivered > values[index-1].Delivered {
			if values[index].AtNanos-lastAdvance > int64(5*time.Second) {
				return nil, fail("S4.3 impaired stream has a zero-delivery interval over five seconds")
			}
			lastAdvance = values[index].AtNanos
		}
	}
	if duration-lastAdvance > int64(5*time.Second) {
		return nil, fail("S4.3 impaired stream ended with a zero-delivery interval over five seconds")
	}
	windows := make([]float64, 0, duration/int64(time.Minute))
	prior := uint32(0)
	for boundary := int64(time.Minute); boundary <= duration; boundary += int64(time.Minute) {
		delivered := progressAt(values, boundary)
		windows = append(windows, float64(delivered-prior)*8/60)
		prior = delivered
	}
	return windows, Result{Verdict: "pass"}
}

func progressAt(values []progressSample, boundary int64) uint32 {
	result := uint32(0)
	for _, value := range values {
		if value.AtNanos > boundary {
			break
		}
		result = value.Delivered
	}
	return result
}

func validTrafficInterval(start, end ResourceSample, delivered uint32) bool {
	if start.AtNanos >= end.AtNanos || delivered == 0 {
		return false
	}
	client := delta(end.ClientReceived, start.ClientReceived) + delta(end.ClientSent, start.ClientSent)
	publisher := delta(end.PublisherReceived, start.PublisherReceived) + delta(end.PublisherSent, start.PublisherSent)
	return float64(client)/float64(delivered) <= 2 && float64(publisher)/float64(delivered) <= 2
}

func verifyShapers(values []shaperEvidence, toolImage string, scope hostScopeEvidence,
	started, completed int64) Result {
	if len(values) != 2 || scope.Adapter != "docker-compose-v1" {
		return invalid("S4.3 requires two independently confined shapers")
	}
	seen, seeds := map[string]bool{}, map[uint32]bool{}
	for _, value := range values {
		var config stressShaperConfigVerification
		var result stressShaperResultVerification
		digest := sha256.Sum256(value.Config)
		observer := value.Observer
		if value.ToolImageID != toolImage || value.ContainerID == "" || value.TargetContainer == "" ||
			value.ConfigDigest != hex.EncodeToString(digest[:]) || value.ReadyObservedAtNanos > started ||
			value.CompletedAtNanos < completed || !value.Removed || seen[value.Role] ||
			decodeAttemptValue(value.Config, 64<<10, &config) != nil || decodeAttemptValue(value.Result, 64<<10, &result) != nil ||
			config.SchemaVersion != "carrier-lab-native-tool-role/v1" || config.Role != value.Role ||
			config.Mode != "shape" || config.Profile != "h3-s43-impaired-v1" || config.Seed == 0 || seeds[config.Seed] ||
			result.SchemaVersion != config.SchemaVersion || result.RunID != config.RunID || result.Role != config.Role ||
			result.Status != "passed" || result.EffectiveCapabilities != "0000000000001000" ||
			len(result.ToolLockSHA256) != 64 || len(result.Qdisc) != 1 || len(result.Tools) != 1 ||
			observer.ContainerID != value.ContainerID || observer.ImageID != toolImage ||
			observer.NetworkMode != "container:"+value.TargetContainer || observer.User != "0:0" ||
			observer.PIDMode != "" || observer.IPCMode != "private" || !observer.ReadOnly || observer.Privileged ||
			observer.MountCount != 4 || observer.PidsLimit != 16 || observer.MemoryLimit != 32<<20 ||
			observer.NanoCPUs != 250_000_000 || !sameStrings(observer.CapAdd, []string{"NET_ADMIN"}) ||
			!sameStrings(observer.CapDrop, []string{"ALL"}) ||
			!sameStrings(observer.SecurityOpt, []string{"no-new-privileges:true"}) {
			return invalid("S4.3 shaper identity, confinement, or lifecycle is invalid")
		}
		for name, tool := range result.Tools {
			if name != "tc" || tool.Path != "/usr/sbin/tc" || tool.Version == "" || len(tool.SHA256) != 64 {
				return invalid("S4.3 shaper tool identity is invalid")
			}
		}
		for _, state := range result.Qdisc {
			if !strings.Contains(state, "delay 150ms") || !strings.Contains(state, "loss 5%") ||
				!strings.Contains(state, "rate 25mbit") || !strings.Contains(state, "limit 1000") {
				return invalid("S4.3 effective qdisc differs from the frozen impaired profile")
			}
		}
		seen[value.Role], seeds[config.Seed] = true, true
	}
	if !seen["shape-client"] || !seen["shape-publisher"] {
		return invalid("S4.3 shaper role set is incomplete")
	}
	return Result{Verdict: "pass"}
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
