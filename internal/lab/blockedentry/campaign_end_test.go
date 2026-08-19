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

func TestFinalCampaignEndAllowsMissingSummaryUntilReducersComplete(t *testing.T) {
	command, decoder := startCampaignEndHelper(t, "clean")
	stderr := make(chan []byte, 1)
	stderr <- nil
	summary, err := finishCollectedCampaign(command, campaignEndStdin{}, decoder, stderr,
		[]finalCellObservation{{ID: "cell"}}, true)
	if err != nil || summary != nil {
		t.Fatalf("missing final summary result=%+v err=%v", summary, err)
	}
}

func TestCampaignEndProcess(t *testing.T) {
	mode := os.Getenv("ARDENTS_CAMPAIGN_END_HELPER")
	if mode == "" {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"schema":"ardents-h3-blocked-campaign-closed-v1"}`)
	if mode == "clean" {
		os.Exit(0)
	}
	if mode == "trailing" {
		_, _ = fmt.Fprintln(os.Stdout, `{}`)
		return
	}
	for {
		time.Sleep(time.Hour)
	}
}

type campaignEndStdin struct{}

func (campaignEndStdin) Write(value []byte) (int, error) { return len(value), nil }
func (campaignEndStdin) Close() error                    { return nil }

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
