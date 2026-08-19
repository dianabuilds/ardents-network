//go:build linux && live

package network_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFinalCampaignTimelineSurvivesWorkerExecs(t *testing.T) {
	if os.Getenv("ARDENTS_FINAL_TIMELINE_HELPER") == "1" {
		origin, err := finalWorkerTimelineOrigin()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("campaign-offset-millis=%d\n", time.Since(origin).Milliseconds())
		return
	}
	campaignOrigin := time.Now().Add(-2 * time.Second)
	anchor, err := finalCampaignMonotonicAnchor(campaignOrigin)
	if err != nil {
		t.Fatal(err)
	}
	first := runFinalTimelineHelper(t, anchor)
	time.Sleep(100 * time.Millisecond)
	second := runFinalTimelineHelper(t, anchor)
	if first < 1_950 || second <= first || second-first > 2_000 {
		t.Fatalf("worker campaign offsets reset across execs: first=%d second=%d", first, second)
	}
}

func runFinalTimelineHelper(t *testing.T, anchor int64) int64 {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run", "^TestFinalCampaignTimelineSurvivesWorkerExecs$", "-test.count=1")
	command.Env = finalWorkerEnvironment(map[string]string{
		"ARDENTS_FINAL_TIMELINE_HELPER":                  "1",
		"ARDENTS_FINAL_CAMPAIGN_MONOTONIC_ANCHOR_MILLIS": strconv.FormatInt(anchor, 10),
	})
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("timeline helper: %v\n%s", err, raw)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if value, found := strings.CutPrefix(line, "campaign-offset-millis="); found {
			offset, parseErr := strconv.ParseInt(value, 10, 64)
			if parseErr == nil {
				return offset
			}
		}
	}
	t.Fatalf("timeline helper omitted its offset: %s", raw)
	return 0
}
