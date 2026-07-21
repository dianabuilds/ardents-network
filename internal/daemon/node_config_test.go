package daemon

import "testing"

func TestDefaultNodeName(t *testing.T) {
	if got := defaultNodeName(""); got != "ardents" {
		t.Fatalf("name = %q, want ardents", got)
	}
	if got := defaultNodeName("custom"); got != "custom" {
		t.Fatalf("name = %q, want custom", got)
	}
}

func TestDefaultDataDir(t *testing.T) {
	if got := defaultDataDir("ardents", ""); got != "var\\ardents" && got != "var/ardents" {
		t.Fatalf("dir = %q, want default var path", got)
	}
	if got := defaultDataDir("ardents", "data"); got != "data" {
		t.Fatalf("dir = %q, want preserved explicit dir", got)
	}
}
