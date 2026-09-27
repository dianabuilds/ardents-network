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
		filepath.Join("internal", "qualification", "streams_linux.go"): {
			"IntroductionInterval   = time.Second",
			"OpenReaderStreams(bounded, 64, ReaderSetupParallelism",
			"verified, err := session.ResolveIntroduction(bounded, destination)",
		},
		filepath.Join("internal", "qualification", "measurements_linux.go"): {
			"IntroductionSpacing = 300 * time.Millisecond",
			"SetupLimit          = 15",
		},
		filepath.Join("internal", "endpoint", "text_publisher_network_linux.go"): {
			"qualificationPublisherOpeningParallelism = qualification.SetupLimit",
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
		if strings.HasSuffix(path, "streams_linux.go") {
			reserve := strings.Index(string(body), "EnsureTokenReserve(setup, recipients.Join")
			prepare := strings.Index(string(body), "PrepareIntroduction(setup, destination")
			if reserve < 0 || prepare < 0 || reserve > prepare {
				t.Fatalf("qualification Reader must reserve slow token work before its ten-second Introduction lifetime")
			}
		}
	}
}
