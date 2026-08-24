package main

import "testing"

func TestReadConfigRejectsMissingFile(t *testing.T) {
	if _, err := readConfig(t.TempDir() + "/missing.json"); err == nil {
		t.Fatal("missing fixture configuration was accepted")
	}
}
