//go:build live

package network_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type blockedCapacityUnit struct {
	index                                int
	endpoint, service, application       string
	routeVolume, applicationVolume, sync string
	released                             time.Time
}

func startBlockedCapacityUnits(t *testing.T, ctx context.Context, project, image, toolImage string,
	fixture blockedEntryFixture, count int,
) []blockedCapacityUnit {
	t.Helper()
	units := make([]blockedCapacityUnit, 0, count)
	publication := project + "_publication"
	for index := range count {
		unit := blockedCapacityUnit{index: index, endpoint: fmt.Sprintf("%s-capacity-endpoint-%02d", project, index),
			service:           fmt.Sprintf("%s-capacity-service-%02d", project, index),
			application:       fmt.Sprintf("%s-capacity-app-%02d", project, index),
			routeVolume:       project + fmt.Sprintf("_capacity_route_%02d", index),
			applicationVolume: project + fmt.Sprintf("_capacity_app_%02d", index),
			sync:              filepath.Join(fixture.root, "sync", fmt.Sprintf("capacity-%02d", index))}
		mustMkdirShared(t, unit.sync)
		createCapacityVolume(t, ctx, project, unit.routeVolume)
		createCapacityVolume(t, ctx, project, unit.applicationVolume)
		createCapacityEndpoint(t, ctx, project, image, fixture, unit)
		if output, err := dockerOutput(ctx, "start", unit.endpoint); err != nil {
			t.Fatalf("start capacity Endpoint %d: %v\n%s", index, err, output)
		}
		startCapacitySidecar(t, ctx, project, image, unit, "observer")
		startCapacitySidecar(t, ctx, project, image, unit, "policy")
		createCapacityApplication(t, ctx, project, image, fixture, unit)
		createCapacityService(t, ctx, project, image, fixture, unit, publication)
		units = append(units, unit)
	}
	arguments := []string{"start"}
	for _, unit := range units {
		arguments = append(arguments, unit.application, unit.service)
	}
	if output, err := dockerOutput(ctx, arguments...); err != nil {
		t.Fatalf("start capacity Application/Service containers: %v\n%s", err, output)
	}
	for index := range units {
		applyFinalContainerBoundary(t, ctx, toolImage, units[index].endpoint,
			finalNetworkBoundary{units[index].endpoint, "203.0.113.8", "20mbit", "40ms", "5ms",
				fmt.Sprintf("%d", 55200+index)})
		units[index].released = time.Now()
		writeLiveFile(t, filepath.Join(units[index].sync, "capacity-start"), []byte("start\n"))
	}
	return units
}

func createCapacityVolume(t *testing.T, ctx context.Context, project, name string) {
	t.Helper()
	if output, err := dockerOutput(ctx, "volume", "create", "--label", "com.docker.compose.project="+project, name); err != nil {
		t.Fatalf("create capacity volume %s: %v\n%s", name, err, output)
	}
}

func capacityContainerBase(action, name, project, image string) []string {
	return []string{action, "--name", name, "--label", "com.docker.compose.project=" + project,
		"--read-only", "--restart", "no", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--user", "65532:65532", "--pids-limit", "128", "--ulimit", "nofile=256:256",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=16m,mode=0700,uid=65532,gid=65532",
		"--tmpfs", "/run/secure:rw,nosuid,nodev,size=8m,mode=0700,uid=65532,gid=65532",
		"--env", "GOMAXPROCS=2", "--env", "GOMEMLIMIT=384MiB", image}
}

func createCapacityEndpoint(t *testing.T, ctx context.Context, project, image string,
	fixture blockedEntryFixture, unit blockedCapacityUnit,
) {
	t.Helper()
	address := fmt.Sprintf("203.0.113.%d", 40+unit.index)
	arguments := capacityContainerBase("create", unit.endpoint, project, image)
	arguments = append(arguments[:len(arguments)-1], "--network", project+"_entry_net", "--ip", address,
		"--cpus", "2", "--memory", "7g", "--memory-swap", "7g",
		"--mount", bindMount(filepath.Join(fixture.root, "input", "endpoint"), "/run/input", true),
		"--mount", bindMount(unit.sync, "/run/evidence", false),
		"--mount", bindMount(fixture.clientBinary, "/candidate/webtunnel-client", true),
		"--mount", volumeMount(unit.routeVolume, "/run/ardents/client-route"),
		"--tmpfs", "/run/state:rw,nosuid,nodev,size=16m,mode=0700,uid=65532,gid=65532",
		"--env", "ARDENTS_BLOCKED_ROLE=endpoint", "--env", "ARDENTS_BLOCKED_PROFILE=final-capacity",
		"--env", "ARDENTS_BLOCKED_ENDPOINT_ADDRESS="+address, "--env", "ARDENTS_DNS_SYNC=/run/evidence",
		"--env", "ARDENTS_BLOCKED_START_FILE=/run/evidence/capacity-start",
		image, "/usr/local/bin/network-live.test", "-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	if output, err := dockerOutput(ctx, arguments...); err != nil {
		t.Fatalf("create capacity Endpoint %d: %v\n%s", unit.index, err, output)
	}
}

func createCapacityApplication(t *testing.T, ctx context.Context, project, image string,
	fixture blockedEntryFixture, unit blockedCapacityUnit,
) {
	t.Helper()
	arguments := capacityContainerBase("create", unit.application, project, image)
	arguments = append(arguments[:len(arguments)-1], "--network", "none", "--cpus", "1", "--memory", "512m",
		"--memory-swap", "512m", "--mount", bindMount(filepath.Join(fixture.root, "input", "client-app"), "/run/input", true),
		"--mount", volumeMount(unit.applicationVolume, "/run/ardents/client-app"),
		"--env", "ARDENTS_BLOCKED_ROLE=client-app", "--env", "ARDENTS_STREAM_LIFETIME=5m")
	if mode := os.Getenv("ARDENTS_CAPACITY_WORKLOAD"); mode != "" {
		if mode != "sustained" {
			t.Fatalf("unsupported capacity workload %q", mode)
		}
		arguments = append(arguments, "--env", "ARDENTS_BLOCKED_WORKLOAD=sustained",
			"--env", "ARDENTS_BLOCKED_SEND_BYTES="+os.Getenv("ARDENTS_CAPACITY_SEND_BYTES"),
			"--env", "ARDENTS_BLOCKED_RECEIVE_BYTES="+os.Getenv("ARDENTS_CAPACITY_RECEIVE_BYTES"),
			"--env", "ARDENTS_STREAM_PROGRESS=1",
			"--env", "ARDENTS_STREAM_CHUNK_DELAY="+os.Getenv("ARDENTS_CAPACITY_CHUNK_DELAY"))
	} else {
		arguments = append(arguments, "--env", "ARDENTS_STREAM_CHUNK_DELAY=1500ms")
	}
	arguments = append(arguments, image,
		"/usr/local/bin/network-live.test", "-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	if output, err := dockerOutput(ctx, arguments...); err != nil {
		t.Fatalf("start capacity Application %d: %v\n%s", unit.index, err, output)
	}
}

func createCapacityService(t *testing.T, ctx context.Context, project, image string,
	fixture blockedEntryFixture, unit blockedCapacityUnit, publication string,
) {
	t.Helper()
	arguments := capacityContainerBase("create", unit.service, project, image)
	arguments = append(arguments[:len(arguments)-1], "--network", "none", "--cpus", "1", "--memory", "512m",
		"--memory-swap", "512m", "--mount", bindMount(filepath.Join(fixture.root, "input", "client-service"), "/run/input", true),
		"--mount", volumeMount(unit.applicationVolume, "/run/ardents/client-app"),
		"--mount", volumeMount(unit.routeVolume, "/run/ardents/client-route"),
		"--mount", volumeMountReadonly(publication, "/run/ardents/publication"), "--env", "ARDENTS_BLOCKED_ROLE=client-service",
		image, "/usr/local/bin/network-live.test", "-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	if output, err := dockerOutput(ctx, arguments...); err != nil {
		t.Fatalf("start capacity Service Endpoint %d: %v\n%s", unit.index, err, output)
	}
}

func bindMount(source, target string, readOnly bool) string {
	value := "type=bind,source=" + filepath.ToSlash(source) + ",target=" + target
	if readOnly {
		value += ",readonly"
	}
	return value
}

func volumeMount(source, target string) string {
	return "type=volume,source=" + source + ",target=" + target
}

func volumeMountReadonly(source, target string) string {
	return volumeMount(source, target) + ",readonly"
}
