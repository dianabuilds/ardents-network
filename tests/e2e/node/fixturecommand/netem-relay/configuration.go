package main

import (
	"errors"
	"flag"
	"io"
	"net"
	"strings"
	"time"
)

const (
	netemDelayMode    = "delay"
	netemDropMode     = "drop"
	netemImpairedMode = "impaired"
)

type relayConfiguration struct {
	listen, target, tc, mode string
	delay                    time.Duration
}

func parseRelayConfiguration(arguments []string) (relayConfiguration, error) {
	flags := flag.NewFlagSet("netem-relay", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configuration := relayConfiguration{}
	flags.StringVar(&configuration.listen, "listen", "", "TCP listen endpoint")
	flags.StringVar(&configuration.target, "target", "", "TCP upstream endpoint")
	flags.StringVar(&configuration.tc, "tc", "/usr/sbin/tc", "absolute tc executable path")
	flags.StringVar(&configuration.mode, "mode", "", "netem mode: delay or drop")
	flags.DurationVar(&configuration.delay, "delay", 0, "positive delay mode duration")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return relayConfiguration{}, errors.New("netem relay arguments are invalid")
	}
	if !validRelayEndpoint(configuration.listen) || !validRelayEndpoint(configuration.target) ||
		!linuxAbsolutePath(configuration.tc) {
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
	case netemImpairedMode:
		if configuration.delay != 0 {
			return relayConfiguration{}, errors.New("impaired netem does not accept a custom delay")
		}
	default:
		return relayConfiguration{}, errors.New("netem relay mode is invalid")
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
	if configuration.mode == netemDelayMode {
		return append(arguments, "delay", configuration.delay.String())
	}
	if configuration.mode == netemImpairedMode {
		return append(arguments, "delay", "20ms", "5ms", "25%", "loss", "5%", "25%", "reorder", "10%", "25%")
	}
	return append(arguments, "loss", "100%")
}
