//go:build ignore

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

type evidenceArtifact struct {
	Name       string `json:"name"`
	SHA256     string `json:"sha256"`
	Bytes      int64  `json:"bytes"`
	Kind       string `json:"kind"`
	Validation string `json:"validation"`
}
type evidenceReproduction struct {
	State       string `json:"state"`
	Artifact    string `json:"command_artifact,omitempty"`
	Count       int    `json:"argument_count,omitempty"`
	Environment string `json:"environment"`
	Association string `json:"association"`
}

type evidencePackage struct {
	Reproduction    evidenceReproduction   `json:"reproduction"`
	BudgetSeconds   float64                `json:"assembly_budget_seconds"`
	Schema          string                 `json:"schema"`
	Assembly        string                 `json:"assembly"`
	At              time.Time              `json:"assembled_at"`
	Root            string                 `json:"evidence_root"`
	Run             runReport              `json:"run"`
	Artifacts       []evidenceArtifact     `json:"artifacts"`
	Association     string                 `json:"association"`
	ConnectionState string                 `json:"connection_state"`
	Connection      *connectionObservation `json:"connection,omitempty"`
	Failures        []string               `json:"assembly_failures"`
}

// Packages are local private indexes, not browser exports. Hashes identify the
// inspected bytes; neither a selected directory nor timing proves causality.
func bundleCommand(args []string) (outcome error) {
	flags := flag.NewFlagSet("bundle", flag.ContinueOnError)
	dir := flags.String("dir", "", "completed private capture directory")
	out := flags.String("out", "", "new private JSON file outside source and capture")
	timeout := flags.Duration("timeout", 10*time.Second, "whole assembly cooperative deadline, positive and at most 30s")
	socket := flags.String("connection-socket", "", "optional explicitly selected owner-private Reader socket")
	var selectedArtifacts []string
	selectedKinds := map[string]string{}
	selectArtifact := func(kind string) func(string) error {
		return func(name string) error {
			if len(selectedArtifacts) >= 8 || name == "" || len(name) > 128 || name == "." || name == ".." || strings.Trim(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-") != "" {
				return errors.New("select at most 8 artifact basenames using ASCII letters, digits, dot, underscore or hyphen")
			}
			if _, exists := selectedKinds[name]; exists {
				return errors.New("duplicate artifact selection")
			}
			selectedArtifacts = append(selectedArtifacts, name)
			selectedKinds[name] = kind
			return nil
		}
	}
	flags.Func("artifact", "explicit opaque private artifact basename; up to 8 selections total", selectArtifact("private-file"))
	flags.Func("profile", "explicit pprof basename; validate offline using selected Go toolchain", selectArtifact("pprof"))
	flags.Func("trace", "explicit Go runtime trace basename; validate offline", selectArtifact("go-trace"))
	var commandArtifact string
	flags.Func("command", "one explicit private JSON argv receipt basename; never executed", func(name string) error {
		if commandArtifact != "" {
			return errors.New("select one command receipt")
		}
		if err := selectArtifact("command-receipt")(name); err != nil {
			return err
		}
		commandArtifact = name
		return nil
	})
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 || !filepath.IsAbs(*dir) || !filepath.IsAbs(*out) || filepath.Clean(*out) != *out {
		return errors.New("select canonical absolute dir and out")
	}
	if *timeout <= 0 || *timeout > 30*time.Second {
		return errors.New("assembly timeout must be positive and at most 30s")
	}
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, *timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	source, err := privateEvidenceRoot(*dir)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, source.Close()) }()
	parent := filepath.Dir(*out)
	outputRoot, err := privateEvidenceRoot(parent)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, outputRoot.Close()) }()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return err
	}
	if within(cwd, *out) || within(*dir, *out) {
		return errors.New("package must be outside source and selected capture")
	}
	names, err := evidenceInputNames(source)
	if err != nil {
		return err
	}
	for _, selected := range selectedArtifacts {
		for _, name := range names {
			if name == selected {
				return errors.New("artifact already included as report input")
			}
		}
	}
	names = append(names, selectedArtifacts...)
	artifacts, err := inventoryEvidence(ctx, source, names, selectedKinds)
	if err != nil {
		return err
	}
	report := assessRunRoot(source, time.Now().UTC())
	if !report.Finished || report.Status == "unavailable" {
		return errors.New("select a valid completed capture")
	}
	packet := evidencePackage{BudgetSeconds: timeout.Seconds(), Schema: "ardents-evidence-package-v1", Assembly: "complete", At: time.Now().UTC(), Root: *dir, Run: report, Artifacts: artifacts, Association: "explicit-input-selection; operation-association-unproven"}
	packet.Reproduction = evidenceReproduction{State: "unselected", Environment: "not-fully-declared", Association: "selected-command-receipt; run-association-unproven"}
	packet.ConnectionState = "unselected"
	packet.Failures = []string{}
	var selectionErr error
	defer func() { outcome = errors.Join(outcome, selectionErr) }()
	for _, gap := range report.Gaps {
		switch gap {
		case "manifest-unavailable-or-invalid", "summary-unavailable-or-invalid", "summary-required-fields-missing", "summary-invalid-values", "summary-invalid-lifetime", "event-history-invalid-or-truncated", "resource-samples-invalid-or-truncated":
			packet.Assembly = "incomplete"
			packet.Failures = append(packet.Failures, gap)
			selectionErr = errors.Join(selectionErr, errors.New("capture input validation failed"))
		}
	}
	if *socket != "" {
		readerErr := validateEvidenceSocket(*socket)
		if readerErr == nil {
			var observation connectionObservation
			observation, readerErr = readConnectionObservation(ctx, *socket)
			if readerErr == nil {
				packet.ConnectionState = "available"
				packet.Connection = &observation
			}
		}
		if readerErr != nil {
			selectionErr = errors.Join(selectionErr, readerErr)
			packet.Assembly = "incomplete"
			packet.ConnectionState = "unavailable"
			packet.Failures = append(packet.Failures, "reader-selection-failed")
		}
	}
	validations := map[string]string{}
	if commandArtifact != "" {
		packet.Reproduction.Artifact = commandArtifact
		count, err := validateEvidenceCommand(source, commandArtifact)
		if err != nil {
			packet.Reproduction.State = "unavailable"
			packet.Assembly = "incomplete"
			packet.Failures = append(packet.Failures, "command-receipt-validation-failed")
			validations[commandArtifact] = "failed"
			selectionErr = errors.Join(selectionErr, err)
		} else {
			packet.Reproduction.State = "available"
			packet.Reproduction.Count = count
			validations[commandArtifact] = "passed"
		}
	}
	for _, name := range selectedArtifacts {
		if selectedKinds[name] != "pprof" && selectedKinds[name] != "go-trace" {
			continue
		}
		if err := validateEvidenceFormat(ctx, source, name, selectedKinds[name]); err != nil {
			validations[name] = "failed"
			packet.Assembly = "incomplete"
			packet.Failures = append(packet.Failures, "profile-validation-failed")
			selectionErr = errors.Join(selectionErr, err)
		} else {
			validations[name] = "passed"
		}
	}
	afterNames, err := evidenceInputNames(source)
	if err != nil {
		return err
	}
	afterNames = append(afterNames, selectedArtifacts...)
	if len(names) != len(afterNames) {
		return errors.New("capture inputs changed while assembling package")
	}
	after, err := inventoryEvidence(ctx, source, afterNames, selectedKinds)
	if err != nil {
		return err
	}
	for i := range artifacts {
		if artifacts[i] != after[i] {
			return errors.New("capture changed while assembling package")
		}
	}
	for i := range packet.Artifacts {
		kind, selected := selectedKinds[packet.Artifacts[i].Name]
		if !selected {
			kind = "report-input"
		}
		packet.Artifacts[i].Kind = kind
		packet.Artifacts[i].Validation = "not-checked"
		if state, checked := validations[packet.Artifacts[i].Name]; checked {
			packet.Artifacts[i].Validation = state
		}
	}
	if err := validateEvidenceAnchor(source, *dir); err != nil {
		return err
	}
	if err := validateEvidenceAnchor(outputRoot, parent); err != nil {
		return err
	}
	packet.At = time.Now().UTC()
	body, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if len(body) > 256<<10 {
		return errors.New("package exceeds 256 KiB")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := outputRoot.OpenFile(filepath.Base(*out), os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	n, werr := file.Write(body)
	if n != len(body) && werr == nil {
		werr = io.ErrShortWrite
	}
	return errors.Join(werr, file.Close(), ctx.Err())
}

func privateEvidenceRoot(dir string) (*os.Root, error) {
	root, err := openEvidenceRoot(dir)
	if err != nil {
		return nil, err
	}
	info, err := root.Stat(".")
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 || !ok || stat.Uid != uint32(os.Geteuid()) {
		return nil, errors.Join(errors.New("select an owned private directory"), root.Close())
	}
	return root, nil
}

func inventoryEvidence(ctx context.Context, root *os.Root, names []string, kinds map[string]string) ([]evidenceArtifact, error) {
	artifacts := make([]evidenceArtifact, 0, len(names))
	var total int64
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		before, err := f.Stat()
		if err != nil {
			return nil, errors.Join(err, f.Close())
		}
		stat, ok := before.Sys().(*syscall.Stat_t)
		limit := int64(64 << 20)
		if kinds[name] == "command-receipt" {
			limit = lineLimit
		}
		if name == "summary.json" {
			limit = recordLimit
		}
		if name == "manifest.json" {
			limit = lineLimit
		}
		if name == "tools.txt" {
			limit = 1 << 20
		}
		if name == "events.ndjson" || name == "samples.ndjson" {
			limit = recordLimit
		}
		if !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 || !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || before.Size() < 0 || before.Size() > limit {
			return nil, errors.Join(errors.New("artifact must be owned private regular file within its budget"), f.Close())
		}
		total += before.Size()
		if total > 128<<20 {
			return nil, errors.Join(errors.New("artifact inventory exceeds 128 MiB total"), f.Close())
		}
		hash := sha256.New()
		n, rerr := io.Copy(hash, inventoryReader{ctx: ctx, reader: io.LimitReader(f, limit+1)})
		after, serr := f.Stat()
		cerr := f.Close()
		if rerr != nil || serr != nil || cerr != nil {
			return nil, errors.Join(rerr, serr, cerr)
		}
		named, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		finalStat, finalOK := after.Sys().(*syscall.Stat_t)
		if !after.Mode().IsRegular() || after.Mode().Perm()&0077 != 0 || !finalOK || finalStat.Uid != uint32(os.Geteuid()) || finalStat.Nlink != 1 || n > limit || n != before.Size() || after.Size() != before.Size() || !before.ModTime().Equal(after.ModTime()) || !os.SameFile(before, named) {
			return nil, errors.New("artifact changed during inventory")
		}
		artifacts = append(artifacts, evidenceArtifact{Name: name, SHA256: hex.EncodeToString(hash.Sum(nil)), Bytes: n})
	}
	return artifacts, nil
}

func evidenceInputNames(root *os.Root) ([]string, error) {
	names := []string{"manifest.json", "summary.json", "events.ndjson", "samples.ndjson"}
	if _, err := root.Lstat("tools.txt"); err == nil {
		names = append(names, "tools.txt")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return names, nil
}

func validateEvidenceSocket(socket string) (outcome error) {
	if !filepath.IsAbs(socket) || filepath.Clean(socket) != socket {
		return errors.New("select a canonical absolute Reader socket")
	}
	canonical, err := filepath.EvalSymlinks(socket)
	if err != nil || canonical != socket {
		return errors.New("Reader socket unavailable or contains symlinks")
	}
	root, err := privateEvidenceRoot(filepath.Dir(socket))
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, root.Close()) }()
	info, err := root.Lstat(filepath.Base(socket))
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0077 != 0 || !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 {
		return errors.New("select an owned private Reader socket")
	}
	return nil
}

// Validation executes only the installed Go parser with a selected file descriptor.
// No symbols, executable, remote URL, shell, raw output or profile execution.
func validateEvidenceFormat(parent context.Context, root *os.Root, name, kind string) (outcome error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || info.Size() < 0 || info.Size() > 64<<20 {
		return errors.New("profile admission failed")
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	parserArgs := []string{"tool", "pprof", "-top", "-nodecount=1", "-symbolize=none", "/proc/self/fd/3"}
	outputLimit := 64 << 10
	if kind == "go-trace" {
		parserArgs = []string{"tool", "trace", "-pprof=sched", "/proc/self/fd/3"}
		outputLimit = 64 << 20
	}
	cmd := exec.CommandContext(ctx, "go", parserArgs...)
	cmd.ExtraFiles = []*os.File{f}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	cmd.Stdout = &evidenceDiscard{remaining: outputLimit}
	cmd.Stderr = &evidenceDiscard{remaining: 64 << 10}
	if err := cmd.Run(); err != nil {
		return errors.Join(errors.New("selected diagnostic format validation failed"), ctx.Err())
	}
	return nil
}

type evidenceDiscard struct{ remaining int }

func (w *evidenceDiscard) Write(body []byte) (int, error) {
	n := min(len(body), w.remaining)
	w.remaining -= n
	if n != len(body) {
		return n, errors.New("profile parser output exceeded budget")
	}
	return n, nil
}

func validateEvidenceAnchor(root *os.Root, path string) (outcome error) {
	current, err := privateEvidenceRoot(path)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, current.Close()) }()
	opened, err := root.Stat(".")
	if err != nil {
		return err
	}
	named, err := current.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(opened, named) {
		return errors.New("evidence directory path changed during assembly")
	}
	return nil
}

func validateEvidenceCommand(root *os.Root, name string) (int, error) {
	body, _, err := readEvidence(root, name, lineLimit)
	if err != nil {
		return 0, err
	}
	var argv []json.RawMessage
	if !utf8.Valid(body) || !validJSONRecord(body) || json.Unmarshal(body, &argv) != nil || len(argv) == 0 || len(argv) > 256 {
		return 0, errors.New("command receipt must be a bounded nonempty JSON string array")
	}
	for i, raw := range argv {
		raw = bytes.TrimSpace(raw)
		var arg string
		if len(raw) < 2 || raw[0] != '"' || json.Unmarshal(raw, &arg) != nil || (i == 0 && arg == "") || strings.ContainsRune(arg, 0) {
			return 0, errors.New("command receipt contains an invalid argument")
		}
	}
	return len(argv), nil
}
