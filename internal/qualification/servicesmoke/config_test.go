package servicesmoke

import (
	"testing"
	"time"
)

func TestParseConfigKeepsBoundedCampaignInputs(t *testing.T) {
	value, err := ParseConfig([]string{"-fixture", "fixture", "-evidence", "evidence", "-compose", "compose",
		"-source", "source", "-duration", "10m"})
	if err != nil || value.Duration != 10*time.Minute || value.FixtureRoot != "fixture" {
		t.Fatalf("unexpected parsed config: value=%+v err=%v", value, err)
	}
	if _, err := ParseConfig([]string{"unexpected"}); err == nil {
		t.Fatal("unexpected positional input was accepted")
	}
}
