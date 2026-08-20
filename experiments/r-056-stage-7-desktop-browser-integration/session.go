//go:build ignore

package main

import "fmt"

type distribution string

const (
	installed distribution = "installed"
	portable  distribution = "portable"
)

type browserMode string

const (
	generic  browserMode = "generic"
	isolated browserMode = "isolated"
)

type carrierPolicy string

const (
	directCarrier  carrierPolicy = "ordinary OS networking"
	vpnCarrier     carrierPolicy = "active VPN permits Carrier"
	blockedCarrier carrierPolicy = "host policy blocks Carrier"
)

type action string

const (
	toggleDistribution  action = "profile"
	toggleRegistration  action = "registration"
	toggleMode          action = "mode"
	toggleBrowser       action = "browser"
	cycleCarrier        action = "carrier"
	connectStream       action = "connect"
	acceptStream        action = "accept"
	directBrowse        action = "browse"
	operatingSystemOpen action = "os-open"
)

type result struct {
	Entry       string
	Class       string
	Claim       string
	Fallback    string
	HostChanges string
	Detail      string
}

type session struct {
	Distribution   distribution
	Registration   bool
	Mode           browserMode
	DefaultBrowser bool
	Carrier        carrierPolicy
	Last           result
}

func newSession() session {
	return session{
		Distribution:   portable,
		Mode:           generic,
		DefaultBrowser: true,
		Carrier:        directCarrier,
	}
}

func apply(current session, requested action) session {
	next := current
	next.Last = result{}
	switch requested {
	case toggleDistribution:
		if next.Distribution == portable {
			next.Distribution = installed
		} else {
			next.Distribution = portable
		}
	case toggleRegistration:
		next.Registration = !next.Registration
		next.Last = registrationResult(next.Registration)
	case toggleMode:
		if next.Mode == generic {
			next.Mode = isolated
		} else {
			next.Mode = generic
		}
	case toggleBrowser:
		next.DefaultBrowser = !next.DefaultBrowser
	case cycleCarrier:
		switch next.Carrier {
		case directCarrier:
			next.Carrier = vpnCarrier
		case vpnCarrier:
			next.Carrier = blockedCarrier
		default:
			next.Carrier = directCarrier
		}
	case connectStream:
		next.Last = streamResult("ardents connect", next.Carrier)
	case acceptStream:
		next.Last = streamResult("ardents accept", next.Carrier)
	case directBrowse:
		next.Last = browseResult("ardents browse", next)
	case operatingSystemOpen:
		if !next.Registration {
			next.Last = stopped("OS ardents:// handoff", "invalid-destination",
				"no per-user Ardents URI handler is selected")
		} else {
			next.Last = browseResult("OS ardents:// handoff", next)
		}
	}
	return next
}

func registrationResult(enabled bool) result {
	detail := "explicit per-user handler removed"
	if enabled {
		detail = "explicit per-user handler registered; default choice remains user/OS-owned"
	}
	return result{
		Entry: "URI registration action", Class: "local-change",
		Claim: "none", Fallback: "none",
		HostChanges: "per-user ardents URI association only", Detail: detail,
	}
}

func streamResult(entry string, carrier carrierPolicy) result {
	if carrier == blockedCarrier {
		return stopped(entry, "evidenced-route-unavailable",
			"current firewall/VPN policy blocks the Endpoint Carrier")
	}
	return result{
		Entry: entry, Class: "connection-interface-result",
		Claim: "bounded direct Application byte stream", Fallback: "none",
		HostChanges: "none", Detail: "browser and URI registration are not consulted",
	}
}

func browseResult(entry string, current session) result {
	if current.Mode == isolated {
		return stopped(entry, "isolation-unsupported",
			"Stage 7 selects no isolated-browser profile; generic fallback is forbidden")
	}
	if current.Carrier == blockedCarrier {
		return stopped(entry, "evidenced-route-unavailable",
			"current firewall/VPN policy blocks the Endpoint Carrier")
	}
	if !current.DefaultBrowser {
		return stopped(entry, "local-resource-denial", "no default browser is available")
	}
	return result{
		Entry: entry, Class: "connection-interface-result",
		Claim: "application-networking-unverified", Fallback: "none",
		HostChanges: "none",
		Detail:      "ephemeral token-bound loopback origin in the existing default browser",
	}
}

func stopped(entry, class, detail string) result {
	return result{
		Entry: entry, Class: class, Claim: "none", Fallback: "none",
		HostChanges: "none", Detail: detail,
	}
}

func (current session) invariantError() error {
	if current.Last.Fallback != "" && current.Last.Fallback != "none" {
		return fmt.Errorf("unexpected fallback %q", current.Last.Fallback)
	}
	if current.Last.HostChanges != "" && current.Last.HostChanges != "none" &&
		current.Last.Entry != "URI registration action" {
		return fmt.Errorf("runtime action mutated host policy: %s", current.Last.HostChanges)
	}
	if current.Last.Claim == "eligible Network-Isolated Application Boundary" {
		return fmt.Errorf("Stage 7 isolated-browser claim must remain unsupported")
	}
	if current.Last.Class == "isolation-unsupported" && current.Mode != isolated {
		return fmt.Errorf("unsupported isolation result escaped isolated mode")
	}
	return nil
}
