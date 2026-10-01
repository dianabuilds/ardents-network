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

func TestQualificationNetworkCellsSupportTCPAndUDPWithFiniteBulkLimit(t *testing.T) {
	for _, network := range []string{"tcp", "udp"} {
		for _, mode := range []string{netemNET14ADDelayMode, netemNET14ADLossMode, netemNET14SDelayMode, netemNET14SLossMode} {
			loss := "0"
			if mode == netemNET14ADLossMode || mode == netemNET14SLossMode {
				loss = "1000"
			}
			configuration, err := parseRelayConfiguration([]string{"-listen", ":47929", "-target", "host:47926", "-network", network, "-mode", mode, "-upstream-rate", "20000000", "-client-rate", "100000000", "-segment-delay", "6667us", "-segment-jitter", "1ms", "-segment-loss-ppm", loss})
			if err != nil {
				t.Fatalf("%s/%s: %v", network, mode, err)
			}
			if configuration.network != network || configuration.directionByteLimit() != 8<<30 || configuration.upstreamRate != 20_000_000 || configuration.clientRate != 100_000_000 {
				t.Fatalf("%s/%s configuration = %+v", network, mode, configuration)
			}
			commands := configuration.trafficControlCommands()
			if len(commands) != 6 || len(commands[3]) < 11 || commands[3][8] != "netem" || commands[3][9] != "delay" ||
				len(commands[4]) < 11 || commands[4][8] != "netem" || commands[4][9] != "delay" {
				t.Fatalf("%s/%s traffic control lost its netem operation: %q", network, mode, commands)
			}
		}
	}
	for _, arguments := range [][]string{
		{"-listen", ":1", "-target", "host:2", "-network", "sctp", "-mode", netemNET14ADLossMode},
		{"-listen", ":1", "-target", "host:2", "-network", "udp", "-mode", netemDelayMode, "-delay", "2s"},
	} {
		if _, err := parseRelayConfiguration(arguments); err == nil {
			t.Fatalf("invalid qualification relay accepted: %q", arguments)
		}
	}
}
