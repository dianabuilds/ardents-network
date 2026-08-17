//go:build live

package network_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type finalNetworkBoundary struct {
	service, target, rate, delay, variation, seed string
}

func applyFinalBlockedNetwork(t *testing.T, ctx context.Context, compose composeCall, toolImage string) {
	t.Helper()
	applyFinalBoundary(t, ctx, compose, toolImage,
		finalNetworkBoundary{"endpoint", "203.0.113.8", "20mbit", "40ms", "5ms", "55001"})
	applyFinalBridgeInfrastructure(t, ctx, compose, toolImage)
}

func applyFinalBridgeInfrastructure(t *testing.T, ctx context.Context, compose composeCall, toolImage string,
	rate ...string,
) {
	t.Helper()
	bridgeRate := "100mbit"
	if len(rate) == 1 {
		bridgeRate = rate[0]
	}
	boundaries := []finalNetworkBoundary{
		{"bridge", "203.0.113.7", bridgeRate, "0ms", "0ms", "55002"},
		{"bridge", "172.31.20.11", bridgeRate, "0ms", "0ms", "55003"},
		{"initiator", "172.31.20.12", "100mbit", "0ms", "0ms", "55004"},
		{"introduction", "172.31.20.13", "100mbit", "0ms", "0ms", "55005"},
		{"rendezvous", "172.31.20.14", "100mbit", "0ms", "0ms", "55006"},
		{"responder", "172.31.20.16", "100mbit", "0ms", "0ms", "55007"},
		{"publisher", "172.31.20.14", "100mbit", "40ms", "5ms", "55008"},
	}
	for _, boundary := range boundaries {
		applyFinalBoundary(t, ctx, compose, toolImage, boundary)
	}
}

func applyFinalDirectNetwork(t *testing.T, ctx context.Context, compose composeCall, toolImage,
	endpointService, publisherService string,
) {
	t.Helper()
	applyFinalBoundary(t, ctx, compose, toolImage,
		finalNetworkBoundary{endpointService, "203.0.113.71", "20mbit", "40ms", "5ms", "55101"})
	applyFinalBoundary(t, ctx, compose, toolImage,
		finalNetworkBoundary{publisherService, "203.0.113.70", "100mbit", "40ms", "5ms", "55102"})
}

func applyFinalBoundary(t *testing.T, ctx context.Context, compose composeCall, toolImage string,
	boundary finalNetworkBoundary,
) {
	t.Helper()
	identity, err := compose(ctx, "ps", "-q", boundary.service)
	if err != nil || strings.TrimSpace(string(identity)) == "" {
		t.Fatalf("resolve final network boundary %s: %v\n%s", boundary.service, err, identity)
	}
	applyFinalContainerBoundary(t, ctx, toolImage, strings.TrimSpace(string(identity)), boundary)
}

func applyFinalContainerBoundary(t *testing.T, ctx context.Context, toolImage, container string,
	boundary finalNetworkBoundary,
) {
	t.Helper()
	deviceOutput, err := dockerOutput(ctx, "run", "--rm", "--network", "container:"+container,
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--entrypoint", "/sbin/ip",
		toolImage, "route", "get", boundary.target)
	if err != nil {
		t.Fatalf("resolve final boundary device for %s: %v\n%s", boundary.service, err, deviceOutput)
	}
	device, err := routeDevice(string(deviceOutput))
	if err != nil {
		t.Fatal(err)
	}
	arguments := []string{"run", "--rm", "--network", "container:" + container, "--user", "0:0",
		"--cap-add", "NET_ADMIN", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--entrypoint", "/usr/sbin/tc", toolImage, "qdisc", "replace", "dev", device, "root", "netem",
		"limit", "10000"}
	if boundary.delay != "0ms" {
		arguments = append(arguments, "delay", boundary.delay, boundary.variation, "distribution", "normal",
			"loss", "0.1%")
	}
	arguments = append(arguments, "rate", boundary.rate, "seed", boundary.seed)
	if output, runErr := dockerOutput(ctx, arguments...); runErr != nil {
		t.Fatalf("apply final boundary %s/%s: %v\n%s", boundary.service, device, runErr, output)
	}
}

func routeDevice(output string) (string, error) {
	fields := strings.Fields(output)
	for index := 0; index+1 < len(fields); index++ {
		if fields[index] == "dev" && fields[index+1] != "" {
			return fields[index+1], nil
		}
	}
	return "", fmt.Errorf("route output has no device: %q", strings.TrimSpace(output))
}
