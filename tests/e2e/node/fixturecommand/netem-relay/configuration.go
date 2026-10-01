package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	netemDelayMode        = "delay"
	netemDropMode         = "drop"
	netemImpairedMode     = "impaired"
	netemNET14ADDelayMode = "net14ad-delay"
	netemNET14ADLossMode  = "net14ad-loss"
	netemNET14SDelayMode  = "net14s-delay"
	netemNET14SLossMode   = "net14s-loss"
)

type relayConfiguration struct {
	listen, target, tc, mode, network string
	delay                             time.Duration
	segmentDelay, segmentJitter       time.Duration
	segmentLossPPM                    uint64
	upstreamRate, clientRate          uint64
}

func parseRelayConfiguration(arguments []string) (relayConfiguration, error) {
	flags := flag.NewFlagSet("netem-relay", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configuration := relayConfiguration{}
	flags.StringVar(&configuration.listen, "listen", "", "TCP listen endpoint")
	flags.StringVar(&configuration.target, "target", "", "TCP upstream endpoint")
	flags.StringVar(&configuration.tc, "tc", "/usr/sbin/tc", "absolute tc executable path")
	flags.StringVar(&configuration.mode, "mode", "", "netem mode: delay or drop")
	flags.StringVar(&configuration.network, "network", "tcp", "bounded relay network: tcp or udp")
	flags.Uint64Var(&configuration.upstreamRate, "upstream-rate", 0, "qualification upstream rate in bit/s")
	flags.Uint64Var(&configuration.clientRate, "client-rate", 0, "qualification client rate in bit/s")
	flags.DurationVar(&configuration.delay, "delay", 0, "positive delay mode duration")
	flags.DurationVar(&configuration.segmentDelay, "segment-delay", 0, "manifest-bound qualification segment delay")
	flags.DurationVar(&configuration.segmentJitter, "segment-jitter", 0, "manifest-bound qualification segment p95 jitter input")
	flags.Uint64Var(&configuration.segmentLossPPM, "segment-loss-ppm", 0, "manifest-bound qualification segment loss")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return relayConfiguration{}, errors.New("netem relay arguments are invalid")
	}
	if !validRelayEndpoint(configuration.listen) || !validRelayEndpoint(configuration.target) ||
		!linuxAbsolutePath(configuration.tc) || configuration.network != "tcp" && configuration.network != "udp" {
		return relayConfiguration{}, errors.New("netem relay endpoints or tool paths are invalid")
	}
	switch configuration.mode {
	case netemDelayMode:
		if configuration.delay < time.Millisecond || configuration.delay > time.Second {
			return relayConfiguration{}, errors.New("netem delay must be between 1ms and 1s")
		}
	case netemDropMode:
		if configuration.delay != 0 {
			return relayConfiguration{}, errors.New("netem drop does not accept a delay")
		}
	case netemImpairedMode, netemNET14ADDelayMode, netemNET14ADLossMode, netemNET14SDelayMode, netemNET14SLossMode:
		if configuration.delay != 0 {
			return relayConfiguration{}, errors.New("impaired netem does not accept a custom delay")
		}
	default:
		return relayConfiguration{}, errors.New("netem relay mode is invalid")
	}
	qualification := configuration.mode == netemNET14ADDelayMode || configuration.mode == netemNET14ADLossMode ||
		configuration.mode == netemNET14SDelayMode || configuration.mode == netemNET14SLossMode
	if qualification != (configuration.upstreamRate != 0 && configuration.clientRate != 0) ||
		configuration.upstreamRate > 100_000_000 || configuration.clientRate > 100_000_000 {
		return relayConfiguration{}, errors.New("qualification relay directional rates are invalid")
	}
	if qualification {
		if _, err := configuration.targetPort(); err != nil {
			return relayConfiguration{}, errors.New("qualification relay target port must be numeric")
		}
		lossMode := configuration.mode == netemNET14ADLossMode || configuration.mode == netemNET14SLossMode
		if configuration.segmentDelay <= 0 || configuration.segmentDelay > 150*time.Millisecond ||
			configuration.segmentJitter < 0 || configuration.segmentJitter > 100*time.Millisecond ||
			configuration.segmentLossPPM > 50_000 || lossMode != (configuration.segmentLossPPM != 0) {
			return relayConfiguration{}, errors.New("qualification relay segment impairment is invalid")
		}
	} else if configuration.segmentDelay != 0 || configuration.segmentJitter != 0 || configuration.segmentLossPPM != 0 {
		return relayConfiguration{}, errors.New("ordinary relay cannot accept qualification segment impairment")
	}
	return configuration, nil
}

func linuxAbsolutePath(value string) bool { return strings.HasPrefix(value, "/") }

func validRelayEndpoint(value string) bool {
	_, port, err := net.SplitHostPort(value)
	return err == nil && port != ""
}

func (configuration relayConfiguration) netemArguments() []string {
	arguments := []string{"qdisc", "replace", "dev", "eth0", "root", "netem"}
	if configuration.segmentDelay != 0 {
		arguments = append(arguments, "delay", configuration.segmentDelay.String())
		if configuration.segmentJitter != 0 {
			arguments = append(arguments, configuration.segmentJitter.String(), "25%")
		}
		if configuration.segmentLossPPM != 0 {
			arguments = append(arguments, "loss", fmt.Sprintf("%.4f%%", float64(configuration.segmentLossPPM)/10_000))
		}
		return arguments
	}
	if configuration.mode == netemDelayMode {
		return append(arguments, "delay", configuration.delay.String())
	}
	if configuration.mode == netemImpairedMode {
		return append(arguments, "delay", "20ms", "5ms", "25%", "loss", "5%", "25%", "reorder", "10%", "25%")
	}
	return append(arguments, "loss", "100%")
}

func (configuration relayConfiguration) trafficControlCommands() [][]string {
	if configuration.upstreamRate == 0 || configuration.clientRate == 0 {
		return [][]string{configuration.netemArguments()}
	}
	_, port, _ := net.SplitHostPort(configuration.target)
	parameters := configuration.netemArguments()[6:]
	return [][]string{
		{"qdisc", "replace", "dev", "eth0", "root", "handle", "1:", "htb", "default", "20"},
		{"class", "add", "dev", "eth0", "parent", "1:", "classid", "1:10", "htb", "rate", rateSpelling(configuration.upstreamRate), "ceil", rateSpelling(configuration.upstreamRate)},
		{"class", "add", "dev", "eth0", "parent", "1:", "classid", "1:20", "htb", "rate", rateSpelling(configuration.clientRate), "ceil", rateSpelling(configuration.clientRate)},
		append([]string{"qdisc", "add", "dev", "eth0", "parent", "1:10", "handle", "10:", "netem"}, parameters...),
		append([]string{"qdisc", "add", "dev", "eth0", "parent", "1:20", "handle", "20:", "netem"}, parameters...),
		{"filter", "add", "dev", "eth0", "protocol", "ip", "parent", "1:", "prio", "1", "u32", "match", "ip", "dport", port, "0xffff", "flowid", "1:10"},
	}
}

func rateSpelling(bits uint64) string { return fmt.Sprintf("%dbit", bits) }

func (configuration relayConfiguration) targetPort() (uint16, error) {
	_, spelling, err := net.SplitHostPort(configuration.target)
	if err != nil {
		return 0, err
	}
	port, err := strconv.ParseUint(spelling, 10, 16)
	return uint16(port), err
}

func (configuration relayConfiguration) directionByteLimit() int64 {
	if configuration.mode == netemNET14ADDelayMode || configuration.mode == netemNET14ADLossMode ||
		configuration.mode == netemNET14SDelayMode || configuration.mode == netemNET14SLossMode {
		return 8 << 30
	}
	return relayDirectionByteLimit
}
