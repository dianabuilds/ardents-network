package recovery

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func s42Samples(terminal int64) []ResourceSample {
	count := int(terminal / int64(time.Second))
	result := make([]ResourceSample, 0, count)
	for index := 1; index <= count; index++ {
		counter := uint64(index * 100)
		result = append(result, ResourceSample{AtNanos: int64(index) * int64(time.Second), ClientRSS: 1,
			PublisherRSS: 1, ClientCPUPercent: 1, PublisherCPUPercent: 1, ClientReceived: counter,
			ClientSent: counter, PublisherReceived: counter, PublisherSent: counter})
	}
	return result
}

func s42Observer(imageID, identity, route string) ObserverProcess {
	return ObserverProcess{ContainerID: identity, ImageID: imageID, NetworkMode: "container:" + route,
		User: "65532:65532", IPCMode: "private",
		Command: []string{"/usr/local/bin/ardents-qualify", "carrier-fault", "traffic-wait"},
		CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges"}, ReadOnly: true, Removed: true,
		PidsLimit: 16, MemoryLimit: 32 << 20, NanoCPUs: 250_000_000}
}

func s42Topology(input []byte) []byte {
	text := strings.Replace(string(input), "  client:\n",
		"  client:\n    volumes: [recovery_introduction_user:/run/ardents/recovery-introduction-user]\n", 1)
	text = strings.Replace(text, "  publisher:\n",
		"  publisher:\n    volumes: [recovery_introduction_service:/run/ardents/recovery-introduction-service]\n", 1)
	var additions strings.Builder
	for _, role := range replacementRoleNames {
		for candidate := 2; candidate <= 3; candidate++ {
			volume := ""
			if role == "introduction" && candidate == 3 {
				volume = "    volumes: [recovery_introduction_user:/run/ardents/recovery-introduction-user, recovery_introduction_service:/run/ardents/recovery-introduction-service]\n"
			}
			fmt.Fprintf(&additions, "  %s-%d:\n    networks: [route_net]\n    restart: \"no\"\n%s", role, candidate, volume)
		}
	}
	return []byte(strings.Replace(text, "networks:\n", additions.String()+"networks:\n", 1))
}

func decodeReplacementTest(t *testing.T, raw []byte) replacementEvidence {
	t.Helper()
	var result replacementEvidence
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func decodeHostScopeTest(t *testing.T, raw []byte) hostScopeEvidence {
	t.Helper()
	value, err := decodeHostScope(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func encodeHostScopeTest(t *testing.T, value hostScopeEvidence) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
