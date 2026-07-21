package daemon

import (
	"ardents/internal/diagnostics"
	transport "ardents/internal/network"
	"reflect"
	"testing"
)

func TestNetworkBootstrapSourcesFiltersMultiaddrs(t *testing.T) {
	got := NetworkBootstrapSources([]string{"local://bootstrap", " /ip4/127.0.0.1/tcp/9000 ", "/dns4/example/tcp/9000"})
	want := []string{"/ip4/127.0.0.1/tcp/9000", "/dns4/example/tcp/9000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %#v, want %#v", got, want)
	}
}

func TestRecordBootstrapDialWritesFailureEvent(t *testing.T) {
	rec := diagnostics.New("")
	RecordBootstrapDial(rec, "node-a", transport.BootstrapDialReport{
		Peer:   "peer-a",
		Detail: "dial timeout",
	})

	snapshot := rec.Snapshot()
	if len(snapshot.RecentEvents) != 1 {
		t.Fatalf("events = %#v, want single event", snapshot.RecentEvents)
	}
	if snapshot.RecentEvents[0].Type != "bootstrap_dial_failed" || snapshot.RecentEvents[0].ReasonCode != "transport.bootstrap.dial_failed" {
		t.Fatalf("event = %#v, want failed bootstrap dial event", snapshot.RecentEvents[0])
	}
}
