//go:build linux

package node

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func nodeProcessTree(mainPID int) (uint64, uint64, uint64, uint64, string, string, error) {
	mainCgroup, err := nodeProcessCgroup(strconv.Itoa(mainPID))
	if err != nil {
		return 0, 0, 0, 0, "", "", err
	}
	directory, err := os.Open("/proc")
	if err != nil {
		return 0, 0, 0, 0, "", "", err
	}
	names, readErr := directory.Readdirnames(8193)
	closeErr := directory.Close()
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if len(names) > 8192 || readErr != nil || closeErr != nil {
		return 0, 0, 0, 0, "", "", errors.Join(readErr, closeErr, errors.New("node proc directory exceeds its bound"))
	}
	fields := make([]string, 0, 512)
	for _, name := range names {
		pid, parseErr := strconv.Atoi(name)
		cgroup, cgroupErr := nodeProcessCgroup(name)
		if parseErr == nil && pid > 0 && cgroupErr == nil && cgroup == mainCgroup {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	if len(fields) == 0 || len(fields) > 512 {
		return 0, 0, 0, 0, "", "", errors.New("node process tree count is invalid")
	}
	var fds, sockets, threads, rss uint64
	var start string
	var evidence strings.Builder
	for _, field := range fields {
		pid, parseErr := strconv.Atoi(field)
		if parseErr != nil || pid < 1 {
			return 0, 0, 0, 0, "", "", errors.New("node process tree PID is invalid")
		}
		processFDs, processSockets, fdErr := nodeFDCounts(pid)
		status, statusErr := byteio.ReadFile(filepath.Join("/proc", field, "status"), 64<<10)
		processThreads, threadOK := nodeStatusCounter(string(status), "Threads:", 1)
		processRSS, rssOK := nodeStatusCounter(string(status), "VmRSS:", 1024)
		if err := errors.Join(fdErr, statusErr); err != nil || !threadOK || !rssOK {
			return 0, 0, 0, 0, "", "", errors.Join(err, errors.New("node process status is invalid"))
		}
		fds, sockets = fds+processFDs, sockets+processSockets
		threads, rss = threads+processThreads, rss+processRSS
		if evidence.Len() > 12<<10 {
			return 0, 0, 0, 0, "", "", errors.New("node process evidence exceeds its bound")
		}
		_, _ = fmt.Fprintf(&evidence, "%d %d %d %d %d\n", pid, processThreads, processRSS, processFDs, processSockets)
		if pid == mainPID {
			start, err = nodeProcessStart(field)
			if err != nil {
				return 0, 0, 0, 0, "", "", err
			}
		}
	}
	if start == "" {
		return 0, 0, 0, 0, "", "", errors.New("node main process is absent from its cgroup")
	}
	return fds, sockets, threads, rss, start, evidence.String(), nil
}

func nodeProcessCgroup(pid string) (string, error) {
	raw, err := byteio.ReadFile(filepath.Join("/proc", pid, "cgroup"), 4096)
	value := strings.TrimSpace(string(raw))
	if err != nil || !strings.HasPrefix(value, "0::/") || strings.Contains(value, "\n") {
		return "", errors.Join(err, errors.New("node process cgroup is invalid"))
	}
	return value, nil
}

func nodeStatusCounter(raw, name string, multiplier uint64) (uint64, bool) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == name {
			value, err := strconv.ParseUint(fields[1], 10, 64)
			return value * multiplier, err == nil
		}
	}
	return 0, false
}

func nodeProcessStart(pid string) (string, error) {
	raw, err := byteio.ReadFile(filepath.Join("/proc", pid, "stat"), 4096)
	if err != nil {
		return "", err
	}
	end := strings.LastIndexByte(string(raw), ')')
	fields := strings.Fields(string(raw)[end+1:])
	if end < 0 || len(fields) <= 19 {
		return "", errors.New("node process start identity is invalid")
	}
	return fields[19], nil
}
