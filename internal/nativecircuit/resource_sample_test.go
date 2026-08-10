package nativecircuit

import "testing"

func TestParseDockerResourceLine(t *testing.T) {
	t.Parallel()
	sample, err := parseDockerResourceLine("abc123\tproject-user-1\t12.50%\t64.5MiB / 512MiB\t1.2MB / 3.4MB\t7")
	if err != nil {
		t.Fatal(err)
	}
	if sample.CPUCores != .125 || sample.RSSBytes != 67633152 || sample.RXBytes != 1_200_000 || sample.TXBytes != 3_400_000 || sample.PIDs != 7 {
		t.Fatalf("unexpected sample: %+v", sample)
	}
}

func TestParseDockerSizeRejectsUnknownUnit(t *testing.T) {
	t.Parallel()
	if _, err := parseDockerSize("1watts"); err == nil {
		t.Fatal("unknown Docker size unit was accepted")
	}
}
