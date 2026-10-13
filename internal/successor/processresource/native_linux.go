//go:build linux

package processresource

import (
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type nativePin struct {
	proc, group           *os.File
	procInfo, groupInfo   os.FileInfo
	groupPath, startClock string
	pid                   int
}

func openNative() (_ *nativePin, result error) {
	proc, err := os.Open("/proc/self")
	if err != nil {
		return nil, err
	}
	pin := &nativePin{proc: proc, pid: os.Getpid()}
	defer func() {
		if result != nil {
			result = errors.Join(result, pin.close())
		}
	}()
	pin.procInfo, err = proc.Stat()
	if err != nil {
		return nil, err
	}
	pin.startClock, err = pin.clock()
	if err != nil {
		return nil, err
	}
	pin.groupPath, err = processGroup()
	if err != nil {
		return nil, err
	}
	pin.group, err = os.Open(pin.groupPath)
	if err != nil {
		return nil, err
	}
	pin.groupInfo, err = pin.group.Stat()
	if err != nil {
		return nil, err
	}
	if err := pin.placement(); err != nil {
		return nil, err
	}
	return pin, nil
}

func processGroup() (string, error) {
	raw, err := nativeRead("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	var relative string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) == 3 && parts[0] == "0" && parts[1] == "" {
			if relative != "" || parts[2] == "/" || !strings.HasPrefix(parts[2], "/") || filepath.Clean(parts[2]) != parts[2] {
				return "", errors.New("original process cgroup unavailable")
			}
			relative = parts[2]
		}
	}
	if relative == "" {
		return "", errors.New("original process cgroup unavailable")
	}
	return filepath.Join("/sys/fs/cgroup", relative), nil
}

func (pin *nativePin) procPath(name string) string {
	return filepath.Join("/proc/self/fd", strconv.Itoa(int(pin.proc.Fd())), name)
}
func (pin *nativePin) groupFile(name string) (string, error) {
	return nativeRead(filepath.Join("/proc/self/fd", strconv.Itoa(int(pin.group.Fd())), name))
}
func (pin *nativePin) clock() (string, error) {
	raw, err := nativeRead(pin.procPath("stat"))
	if err != nil {
		return "", err
	}
	end := strings.LastIndex(raw, ")")
	if end < 0 {
		return "", errors.New("original process clock unavailable")
	}
	fields := strings.Fields(raw[end+1:])
	if len(fields) < 20 {
		return "", errors.New("original process clock unavailable")
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", err
	}
	return fields[19], nil
}

func (pin *nativePin) placement() error {
	if pin == nil || pin.proc == nil || pin.group == nil || os.Getpid() != pin.pid {
		return errors.New("original native process absent")
	}
	procInfo, err := os.Stat("/proc/self")
	if err != nil || !os.SameFile(procInfo, pin.procInfo) {
		return errors.Join(errors.New("original process replaced"), err)
	}
	clock, err := pin.clock()
	if err != nil || clock != pin.startClock {
		return errors.Join(errors.New("original process clock changed"), err)
	}
	groupPath, err := processGroup()
	if err != nil || groupPath != pin.groupPath {
		return errors.Join(errors.New("original process placement changed"), err)
	}
	groupInfo, err := os.Stat(groupPath)
	if err != nil || !os.SameFile(groupInfo, pin.groupInfo) {
		return errors.Join(errors.New("original process cgroup replaced"), err)
	}
	entries, err := pin.group.ReadDir(128)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) == 128 {
		return errors.New("original cgroup inventory exceeds bound")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return errors.New("process cgroup contains a descendant")
		}
	}
	if _, err := pin.group.Seek(0, 0); err != nil {
		return err
	}
	for name, want := range map[string]string{"cpu.max": "100000 100000", "memory.high": "805306368", "memory.max": "1073741824", "pids.max": "256", "cgroup.procs": strconv.Itoa(pin.pid)} {
		raw, err := pin.groupFile(name)
		if err != nil || strings.TrimSpace(raw) != want {
			return errors.Join(errors.New("original process profile placement differs"), err)
		}
	}
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil || limit.Cur != 1024 || limit.Max != 1024 {
		return errors.Join(errors.New("original process file limit differs"), err)
	}
	if runtime.GOMAXPROCS(0) != 1 {
		return errors.New("original Go processor limit differs")
	}
	samples := []metrics.Sample{{Name: "/gc/gomemlimit:bytes"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() != metrics.KindUint64 || samples[0].Value.Uint64() != 512<<20 {
		return errors.New("original Go memory limit differs")
	}
	return nil
}

func (pin *nativePin) observe() (Observation, error) {
	observation := Observation{Started: time.Now()}
	if err := pin.placement(); err != nil {
		return observation, err
	}
	files := make(map[string]string)
	for _, name := range []string{"cpu.stat", "memory.current", "memory.events.local", "pids.events", "cpu.pressure", "memory.pressure", "io.pressure"} {
		raw, err := pin.groupFile(name)
		if err != nil {
			return observation, err
		}
		files[name] = raw
	}
	var err error
	observation.CPUUsec, err = nativeCounter(files["cpu.stat"], "usage_usec")
	if err != nil {
		return observation, err
	}
	observation.MemoryBytes, err = strconv.ParseUint(strings.TrimSpace(files["memory.current"]), 10, 64)
	if err != nil {
		return observation, err
	}
	observation.HighEvents, err = nativeCounter(files["memory.events.local"], "high")
	if err != nil {
		return observation, err
	}
	for _, name := range []string{"max", "oom", "oom_kill", "oom_group_kill"} {
		value, err := nativeCounter(files["memory.events.local"], name)
		if err != nil || value > math.MaxUint64-observation.EmergencyEvents {
			return observation, errors.Join(errors.New("native memory event invalid"), err)
		}
		observation.EmergencyEvents += value
	}
	observation.PIDEvents, err = nativeCounter(files["pids.events"], "max")
	if err != nil {
		return observation, err
	}
	observation.CPUPressure, err = nativePressure(files["cpu.pressure"], "some")
	if err != nil {
		return observation, err
	}
	observation.MemoryPressure, err = nativePressure(files["memory.pressure"], "some")
	if err != nil {
		return observation, err
	}
	observation.IOPressure, err = nativePressure(files["io.pressure"], "full")
	if err != nil {
		return observation, err
	}
	observation.FDs, err = nativeCount(pin.procPath("fd"), 1024)
	if err != nil {
		return observation, err
	}
	observation.Threads, err = nativeCount(pin.procPath("task"), 256)
	if err != nil {
		return observation, err
	}
	statm, err := nativeRead(pin.procPath("statm"))
	if err != nil {
		return observation, err
	}
	fields := strings.Fields(statm)
	if len(fields) != 7 {
		return observation, errors.New("native process RSS unavailable")
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	pageSize := uint64(os.Getpagesize())
	if err != nil || pages > math.MaxUint64/pageSize {
		return observation, errors.Join(errors.New("native process RSS invalid"), err)
	}
	observation.RSSBytes = pages * pageSize
	samples := []metrics.Sample{{Name: "/memory/classes/total:bytes"}, {Name: "/memory/classes/heap/released:bytes"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() != metrics.KindUint64 || samples[1].Value.Kind() != metrics.KindUint64 || samples[0].Value.Uint64() < samples[1].Value.Uint64() {
		return observation, errors.New("native Go memory unavailable")
	}
	observation.GoMemoryBytes = samples[0].Value.Uint64() - samples[1].Value.Uint64()
	if err := pin.placement(); err != nil {
		return observation, err
	}
	observation.Completed = time.Now()
	return observation, nil
}

func (pin *nativePin) close() error {
	if pin == nil {
		return nil
	}
	var result error
	if pin.group != nil {
		result = pin.group.Close()
	}
	if pin.proc != nil {
		result = errors.Join(result, pin.proc.Close())
	}
	return result
}

func nativeRead(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	raw, err := io.ReadAll(io.LimitReader(f, 16385))
	err = errors.Join(err, f.Close())
	if len(raw) > 16384 {
		return "", errors.New("native observation exceeds bound")
	}
	return string(raw), err
}
func nativeCount(path string, maximum int) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	names, err := f.Readdirnames(maximum + 1)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	err = errors.Join(err, f.Close())
	if len(names) > maximum {
		return 0, errors.New("native process inventory exceeds bound")
	}
	return uint64(len(names)), err
}
