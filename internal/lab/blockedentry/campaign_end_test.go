package blockedentry

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestCampaignEndRejectsTrailingEvidence(t *testing.T) {
	command, decoder := startCampaignEndHelper(t, "trailing")
	_, readErr, _ := waitForCampaignEnd(command, decoder, time.Second)
	if readErr == nil {
		t.Fatal("trailing campaign evidence was accepted")
	}
}

func TestCampaignEndKillsAndReapsRunnerThatDoesNotExit(t *testing.T) {
	command, decoder := startCampaignEndHelper(t, "hang")
	started := time.Now()
	_, readErr, waitErr := waitForCampaignEnd(command, decoder, 100*time.Millisecond)
	if readErr == nil || waitErr == nil {
		t.Fatalf("unbounded runner readErr=%v waitErr=%v", readErr, waitErr)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("runner kill and reap exceeded the test bound")
	}
}

func TestCampaignEndProcess(t *testing.T) {
	mode := os.Getenv("ARDENTS_CAMPAIGN_END_HELPER")
	if mode == "" {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"schema":"ardents-h3-blocked-campaign-closed-v1"}`)
	if mode == "trailing" {
		_, _ = fmt.Fprintln(os.Stdout, `{}`)
		return
	}
	for {
		time.Sleep(time.Hour)
	}
}

func startCampaignEndHelper(t *testing.T, mode string) (*exec.Cmd, *json.Decoder) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=TestCampaignEndProcess")
	command.Env = append(os.Environ(), "ARDENTS_CAMPAIGN_END_HELPER="+mode)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	return command, json.NewDecoder(stdout)
}
