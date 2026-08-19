//go:build live

package network_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type finalTreeSample struct {
	Schema       string  `json:"schema"`
	Ordinal      uint16  `json:"ordinal"`
	OffsetMillis uint64  `json:"offset_millis"`
	CPUCores     float64 `json:"cpu_cores"`
	RSSBytes     uint64  `json:"rss_bytes"`
}

func writeFinalRuntimeTrees(t *testing.T, root string, timeline time.Time, samples []finalRuntimeSample) {
	t.Helper()
	values := map[string][]finalTreeSample{"endpoint": {}, "bridge": {}, "publisher": {}}
	for index, sample := range samples {
		offset := uint64(sample.at.Sub(timeline).Milliseconds())
		values["endpoint"] = append(values["endpoint"], finalTreeSample{Schema: "ardents-h3-tree-resource-v1",
			Ordinal: uint16(index), OffsetMillis: offset, CPUCores: sample.endpointCPU,
			RSSBytes: uint64(sample.endpointRSS)})
		values["bridge"] = append(values["bridge"], finalTreeSample{Schema: "ardents-h3-tree-resource-v1",
			Ordinal: uint16(index), OffsetMillis: offset, CPUCores: sample.bridgeCPU,
			RSSBytes: uint64(sample.bridgeRSS)})
		values["publisher"] = append(values["publisher"], finalTreeSample{Schema: "ardents-h3-tree-resource-v1",
			Ordinal: uint16(index), OffsetMillis: offset, CPUCores: sample.publisherCPU,
			RSSBytes: uint64(sample.publisherRSS)})
	}
	for role, series := range values {
		writeFinalTreeSeries(t, filepath.Join(root, "sync", role, "tree.jsonl"), series)
	}
}

func writeFinalCapacityTreeSnapshot(t *testing.T, ctx context.Context, path string,
	timeline time.Time, containers ...string,
) finalTreeSample {
	t.Helper()
	sample := readFinalTreeSnapshot(t, ctx, containers)
	sample.Schema, sample.OffsetMillis = "ardents-h3-tree-resource-v1",
		uint64(time.Since(timeline).Milliseconds())
	writeFinalTreeSeries(t, path, []finalTreeSample{sample})
	return sample
}

func readFinalTreeSnapshot(t *testing.T, ctx context.Context, containers []string) finalTreeSample {
	t.Helper()
	identities := make(map[string]bool, len(containers))
	arguments := []string{"stats", "--no-stream", "--format", "{{.ID}}|{{.CPUPerc}}|{{.MemUsage}}"}
	for _, name := range containers {
		output, err := dockerOutput(ctx, "inspect", "--format", "{{.Id}}", name)
		if err != nil || len(bytes.TrimSpace(output)) != 64 {
			t.Fatalf("resolve final resource tree %s: %v\n%s", name, err, output)
		}
		identities[string(bytes.TrimSpace(output))] = true
		arguments = append(arguments, name)
	}
	output, err := dockerOutput(ctx, arguments...)
	if err != nil {
		t.Fatalf("sample final resource tree: %v\n%s", err, output)
	}
	var result finalTreeSample
	seen := make(map[string]bool, len(containers))
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte{'\n'}) {
		fields := bytes.Split(line, []byte{'|'})
		if len(fields) != 3 {
			t.Fatalf("invalid final tree sample %q", line)
		}
		cpu, cpuErr := parseFinalCPU(string(fields[1]))
		rss, rssErr := liveQuantity(string(bytes.TrimSpace(bytes.Split(fields[2], []byte{'/'})[0])))
		identity := string(fields[0])
		matched := ""
		for expected := range identities {
			if len(identity) <= len(expected) && expected[:len(identity)] == identity {
				matched = expected
				break
			}
		}
		if cpuErr != nil || rssErr != nil || matched == "" || seen[matched] {
			t.Fatalf("invalid final tree values %q: %v", line, errors.Join(cpuErr, rssErr))
		}
		seen[matched] = true
		result.CPUCores += cpu
		result.RSSBytes += uint64(rss)
	}
	if len(seen) != len(containers) {
		t.Fatalf("final resource tree sample=%d want=%d", len(seen), len(containers))
	}
	return result
}

func parseFinalCPU(value string) (float64, error) {
	var percent float64
	if _, err := fmtSscanfPercent(value, &percent); err != nil {
		return 0, err
	}
	return percent / 100, nil
}

func fmtSscanfPercent(value string, percent *float64) (int, error) {
	return fmt.Sscanf(value, "%f%%", percent)
}

func writeFinalTreeSeries(t *testing.T, path string, values []finalTreeSample) {
	t.Helper()
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	for _, value := range values {
		if err := encoder.Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	if len(values) == 0 || output.Len() > maximumFinalTelemetryFile {
		t.Fatal("final resource tree series is empty or oversized")
	}
	if err := os.WriteFile(path, output.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
