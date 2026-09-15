//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type candidateEnvironment struct {
	Host         string
	OSRelease    string
	Kernel       string
	BootID       string
	Cgroup       string
	Systemd      string
	Architecture string
	LogicalCPUs  int
}

func inQualificationEndpointCgroup(observation string) bool {
	expected := "0::" + qualificationOwnerControlGroup + "/ardents-endpoint.service"
	for _, record := range strings.Split(observation, "\n") {
		if strings.TrimSpace(record) == expected {
			return true
		}
	}
	return false
}

func readCandidateEnvironment(ctx context.Context) (candidateEnvironment, error) {
	var environment candidateEnvironment
	var err error
	environment.Host, err = os.Hostname()
	if err != nil {
		return environment, err
	}
	for _, field := range []struct {
		path  string
		limit int64
		value *string
	}{
		{"/usr/lib/os-release", 8192, &environment.OSRelease},
		{"/proc/sys/kernel/osrelease", 256, &environment.Kernel},
		{"/proc/sys/kernel/random/boot_id", 64, &environment.BootID},
		{"/proc/self/cgroup", 8192, &environment.Cgroup},
	} {
		file, err := os.Open(field.path)
		if err != nil {
			return environment, err
		}
		body, readErr := io.ReadAll(io.LimitReader(file, field.limit+1))
		if err := errors.Join(readErr, file.Close()); err != nil {
			return environment, err
		}
		if len(body) == 0 || int64(len(body)) > field.limit {
			return environment, errors.New("candidate environment observation invalid")
		}
		*field.value = strings.TrimSpace(string(body))
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	body, err := exec.CommandContext(bounded, "/usr/bin/systemctl", "--version").Output()
	if err != nil || len(body) == 0 || len(body) > 8192 {
		return environment, errors.New("candidate manager identity unavailable")
	}
	environment.Systemd = strings.TrimSpace(string(body))
	environment.Architecture, environment.LogicalCPUs = runtime.GOARCH, runtime.NumCPU()
	return environment, nil
}
