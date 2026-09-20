package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualificationSmokeAllowsCompleteRetainedSetup(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	requiredByFile := map[string][]string{
		filepath.Join("internal", "endpoint", "stream_qualification_connections_linux.go"): {
			"qualificationIntroductionInterval   = time.Second",
			"openQualificationReaderStreams(bounded, 64, qualificationReaderSetupParallelism",
			"verified, err := owner.resolveTextIntroduction(bounded, worker.job, destination)",
		},
		filepath.Join("internal", "endpoint", "stream_qualification_measurements_linux.go"): {
			"streamQualificationIntroductionSpacing = 300 * time.Millisecond",
			"streamQualificationSetupLimit          = 15",
		},
		filepath.Join("internal", "endpoint", "text_publisher_network_linux.go"): {
			"qualificationPublisherOpeningParallelism = streamQualificationSetupLimit",
			"qualificationPublisherOpeningBatch = 16",
			"ensureQualificationPublisherJoinReserve(network, 32)",
		},
		filepath.Join("tests", "qualification", "stream-network-two-host", "run-windows.ps1"): {
			"$smokeDeadline = [DateTime]::UtcNow.AddMinutes(6)",
			"$deadline = [DateTime]::UtcNow.AddMinutes($(if ($SmokeSeconds -gt 0) { 10 } else { 22 }))",
		},
	}
	for path, required := range requiredByFile {
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range required {
			if !strings.Contains(string(body), value) {
				t.Fatalf("qualification smoke capacity lost %q from %s", value, path)
			}
		}
		if strings.HasSuffix(path, "text_publisher_network_linux.go") && strings.Contains(string(body), "var setup sync.Mutex") {
			t.Fatal("qualification Publisher must not serialize already delivered ten-second Introduction capsules")
		}
		if strings.HasSuffix(path, "stream_qualification_connections_linux.go") {
			reserve := strings.Index(string(body), "ensureQualificationTokenReserve(setup, joinReceiver")
			prepare := strings.Index(string(body), "prepareResolvedTextIntroduction(setup, worker.job")
			if reserve < 0 || prepare < 0 || reserve > prepare {
				t.Fatalf("qualification Reader must reserve slow token work before its ten-second Introduction lifetime")
			}
		}
	}
}
