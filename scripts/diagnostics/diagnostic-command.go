//go:build ignore

// Local Linux diagnostic runner. This is engineering tooling, not a product command.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func dispatch(args []string) error {
	if len(args) == 0 {
		return errors.New("use doctor, bundle, monitor, run, test, static, analyze, snapshot, connection, timings, report, or serve; see scripts/diagnostics/README.md")
	}
	switch args[0] {
	case "bundle":
		return bundleCommand(args[1:])
	case "monitor":
		return monitorCommand(args[1:])
	case "report":
		return reportCommand(args[1:])
	case "doctor":
		return doctor()
	case "serve":
		return serve(args[1:])
	case "connection":
		return connectionCommand(args[1:])
	case "snapshot":
		return snapshotCommand(args[1:])
	case "timings":
		return testTimings(args[1:])
	case "analyze":
		return analyze(args[1:])
	case "run", "test", "static":
		return collect(args[0], args[1:])
	default:
		return errors.New("unknown diagnostic operation")
	}
}

func doctor() error {
	failed := false
	for _, tool := range []struct {
		name string
		args []string
	}{
		{"go", []string{"version"}}, {"staticcheck", []string{"-version"}},
		{"govulncheck", []string{"-version"}}, {"dlv", []string{"version"}},
		{"go", []string{"version", "-m", "/go/bin/errcheck"}}, {"strace", []string{"-V"}}, {"ss", []string{"-V"}}, {"tcpdump", []string{"--version"}},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		body, err := exec.CommandContext(ctx, tool.name, tool.args...).CombinedOutput()
		cancel()
		fmt.Printf("%s: %s\n", tool.name, strings.TrimSpace(string(body)))
		if err != nil {
			failed = true
			fmt.Printf("unavailable: %v\n", err)
		}
	}
	if runtime.GOOS != "linux" {
		failed = true
	}
	if failed {
		return errors.New("diagnostic environment incomplete")
	}
	return nil
}

func collect(mode string, args []string) error {
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	out := flags.String("out", "", "new absolute evidence directory outside source")
	timeout := flags.Duration("timeout", 2*time.Minute, "whole command time, including compilation")
	raw := flags.Bool("raw", false, "retain sensitive stdout/stderr (16 MiB each)")
	pkg := flags.String("package", "./internal/route", "one Go package for test/profile")
	pattern := flags.String("run", "", "explicit test regexp")
	bench := flags.String("bench", "", "explicit benchmark regexp")
	race := flags.Bool("race", false, "race detector; separate from profiling")
	profile := flags.Bool("profile", false, "CPU, heap, block, mutex profiles and runtime trace")
	tags := flags.String("tags", "", "explicit build tags")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *timeout <= 0 || *timeout > 24*time.Hour {
		return errors.New("timeout must be positive and at most 24h")
	}
	if !filepath.IsAbs(*out) {
		return errors.New("out must be absolute")
	}
	absolute, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return err
	}
	absolute = filepath.Join(parent, filepath.Base(absolute))
	if within(resolvedRoot, absolute) {
		return errors.New("evidence must be outside source")
	}
	command := flags.Args()
	if mode == "test" {
		if len(command) != 0 || !strings.HasPrefix(*pkg, "./") || strings.Contains(*pkg, "...") || strings.Contains(*pkg, "..") || strings.Contains(*pkg, "/../") || strings.Contains(*pkg, "\\") || (*pattern == "" && *bench == "") {
			return errors.New("select one ./package and an explicit -run or -bench")
		}
		if *race && *profile {
			return errors.New("collect race and profiling in separate runs")
		}
		command = []string{"go", "test", "-json", "-count=1", "-timeout=" + timeout.String(), "-o", filepath.Join(absolute, "test.bin")}
		if *pattern != "" {
			command = append(command, "-run", *pattern)
		} else {
			command = append(command, "-run", "^$")
		}
		if *bench != "" {
			command = append(command, "-bench", *bench, "-benchtime=1s")
		}
		if *race {
			command = append(command, "-race")
		}
		if *tags != "" {
			command = append(command, "-tags", *tags)
		}
		if *profile {
			command = append(command, "-cpuprofile", filepath.Join(absolute, "cpu.pprof"), "-memprofile", filepath.Join(absolute, "heap.pprof"), "-blockprofile", filepath.Join(absolute, "block.pprof"), "-blockprofilerate=1", "-mutexprofile", filepath.Join(absolute, "mutex.pprof"), "-mutexprofilefraction=1", "-trace", filepath.Join(absolute, "runtime.trace"))
		}
		command = append(command, *pkg)
	}
	if mode == "static" {
		if len(command) != 0 {
			return errors.New("static takes no positional command")
		}
		command = []string{"sh", "-c", "go vet ./...; a=$?; staticcheck -checks 'SA*,S1*,QF*' ./...; b=$?; staticcheck -checks 'SA*,S1*,QF*' -tags text_worker_installed ./...; c=$?; errcheck -blank -asserts -ignoretests ./...; d=$?; test $a -eq 0 && test $b -eq 0 && test $c -eq 0 && test $d -eq 0"}
		*raw = true // Analyzer text is evidence, never projected into metrics.
	}
	if len(command) == 0 {
		return errors.New("run requires -- command arguments")
	}
	conditions := reportConditions{Mode: mode}
	if mode == "test" {
		conditions.Race = race
		conditions.Profiling = profile
	}
	return superviseWithConditions(absolute, root, command, *timeout, *raw, conditions)
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func writeJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(body, '\n'))
	return errors.Join(err, f.Close())
}

func analyze(args []string) error {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	dir := flags.String("dir", "", "evidence directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !filepath.IsAbs(*dir) || len(flags.Args()) != 0 {
		return errors.New("select absolute -dir")
	}
	for _, name := range []string{"cpu", "heap", "block", "mutex"} {
		path := filepath.Join(*dir, name+".pprof")
		if _, err := os.Stat(path); err != nil {
			return err
		}
		body, err := exec.Command("go", "tool", "pprof", "-top", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s profile: %w", name, err)
		}
		fmt.Printf("%s\n%s\n", name, body)
	}
	return nil
}
