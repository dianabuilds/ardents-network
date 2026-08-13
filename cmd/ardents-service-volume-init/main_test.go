package main

import "testing"

func TestVolumeInitializerRejectsRootAndMalformedInputs(t *testing.T) {
	for _, arguments := range [][]string{nil, {"0:0", t.TempDir()}, {"user", t.TempDir()}, {"1:1", "missing"}} {
		if err := run(arguments); err == nil {
			t.Fatalf("unsafe volume input accepted: %v", arguments)
		}
	}
}
