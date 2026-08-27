package main

import (
	"reflect"
	"testing"
	"time"
)

func TestParseRelayConfiguration(t *testing.T) {
	configuration, err := parseRelayConfiguration([]string{"-listen", ":47929", "-target", "82.23.173.198:47926", "-mode", "delay", "-delay", "200ms"})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.delay != 200*time.Millisecond || configuration.mode != netemDelayMode {
		t.Fatalf("configuration = %#v", configuration)
	}
	if got, want := configuration.netemArguments(), []string{"qdisc", "replace", "dev", "eth0", "root", "netem", "delay", "200ms"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("netem arguments = %q, want %q", got, want)
	}
}

func TestParseRelayConfigurationRejectsUnsafeOrAmbiguousInput(t *testing.T) {
	for _, arguments := range [][]string{
		{"-listen", ":47929", "-target", "host:47926", "-mode", "delay"},
		{"-listen", ":47929", "-target", "host:47926", "-mode", "delay", "-delay", "2s"},
		{"-listen", ":47929", "-target", "host:47926", "-mode", "drop", "-delay", "1ms"},
		{"-listen", ":47929", "-target", "host:47926", "-mode", "drop", "-tc", "tc"},
		{"-listen", ":47929", "-target", "host:47926", "-mode", "reorder"},
	} {
		if _, err := parseRelayConfiguration(arguments); err == nil {
			t.Fatalf("parseRelayConfiguration(%q) unexpectedly succeeded", arguments)
		}
	}
}

func TestImpairedRelayConfigurationFixesEveryFaultParameter(t *testing.T) {
	configuration, err := parseRelayConfiguration([]string{"-listen", ":47929", "-target", "host:47926", "-mode", netemImpairedMode})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"qdisc", "replace", "dev", "eth0", "root", "netem", "delay", "20ms", "5ms", "25%", "loss", "5%", "25%", "reorder", "10%", "25%"}
	if got := configuration.netemArguments(); !reflect.DeepEqual(got, want) {
		t.Fatalf("impaired netem arguments = %q, want %q", got, want)
	}
}
