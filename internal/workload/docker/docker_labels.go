package docker

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const (
	labelManaged    = "io.ardents.managed"
	labelSchema     = "io.ardents.schema"
	labelNode       = "io.ardents.node"
	labelWorkload   = "io.ardents.workload"
	labelGeneration = "io.ardents.generation"
	labelRuntime    = "io.ardents.runtime"
	labelTrust      = "io.ardents.trust"
	labelProxy      = "io.ardents.ingress-proxy"
	labelIngress    = "io.ardents.ingress"
)

func workloadLabels(nodeID, workloadID, runtime, trustClass string, generation int64) map[string]string {
	return map[string]string{
		labelManaged:    "true",
		labelSchema:     "1",
		labelNode:       nodeID,
		labelWorkload:   workloadID,
		labelGeneration: strconv.FormatInt(generation, 10),
		labelRuntime:    runtime,
		labelTrust:      trustClass,
	}
}

func generationFromLabels(labels map[string]string) (int64, error) {
	generation, err := strconv.ParseInt(labels[labelGeneration], 10, 64)
	if err != nil || generation <= 0 {
		return 0, fmt.Errorf("invalid Ardents workload generation label")
	}
	return generation, nil
}

func containerName(workloadID string, generation int64) string {
	var name strings.Builder
	name.WriteString("ardents-")
	for _, char := range strings.ToLower(workloadID) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.' {
			name.WriteRune(char)
			continue
		}
		name.WriteByte('-')
	}
	cleaned := strings.Trim(name.String(), "-_.")
	if len(cleaned) > 80 {
		cleaned = cleaned[:80]
	}
	return fmt.Sprintf("%s-%d", cleaned, generation)
}
