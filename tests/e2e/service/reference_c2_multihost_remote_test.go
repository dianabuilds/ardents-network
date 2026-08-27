//go:build h4_3b_multihost

package service_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const h43RemoteImage = "golang:1.26.6"

type h43MultiHostEnvironment struct {
	host, sshKey, sshPath, user, knownHosts string
	port                                    int
	remoteDirectory, container              string
}

func requireH43MultiHostEnvironment(t *testing.T) h43MultiHostEnvironment {
	t.Helper()
	host := os.Getenv("ARDENTS_H4_3B_VPS")
	if ip := net.ParseIP(host); ip == nil || ip.IsUnspecified() {
		t.Fatal("ARDENTS_H4_3B_VPS must be one non-unspecified literal VPS IP address")
	}
	sshKey := os.Getenv("ARDENTS_H4_3B_SSH_KEY")
	if info, err := os.Stat(sshKey); err != nil || !info.Mode().IsRegular() {
		t.Fatal("ARDENTS_H4_3B_SSH_KEY must name an existing private-key file")
	}
	sshPath := os.Getenv("ARDENTS_H4_3B_SSH")
	if sshPath == "" {
		var err error
		for _, name := range []string{"ssh", "ssh.exe"} {
			sshPath, err = exec.LookPath(name)
			if err == nil {
				break
			}
		}
		if sshPath == "" {
			t.Fatal("H4-3B multi-host qualification requires the ssh command")
		}
	}
	if info, err := os.Stat(sshPath); err != nil || info.IsDir() {
		t.Fatal("ARDENTS_H4_3B_SSH must name an existing ssh executable")
	}
	port := 48026
	if value := os.Getenv("ARDENTS_H4_3B_VPS_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1024 || parsed > 65527 {
			t.Fatal("ARDENTS_H4_3B_VPS_PORT must leave seven following unprivileged test ports")
		}
		port = parsed
	}
	user := os.Getenv("ARDENTS_H4_3B_VPS_USER")
	if user == "" {
		user = "root"
	}
	if strings.Trim(user, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") != "" {
		t.Fatal("ARDENTS_H4_3B_VPS_USER contains unsupported characters")
	}
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(nonce[:])
	return h43MultiHostEnvironment{host: host, sshKey: sshKey, sshPath: sshPath, user: user,
		knownHosts: filepath.Join(t.TempDir(), "known_hosts"), port: port,
		remoteDirectory: "/tmp/ardents-h4-3b-multihost-" + suffix, container: "ardents-h4-3b-multihost-" + suffix}
}

type h43RemoteC2 struct{ environment h43MultiHostEnvironment }

func (remote h43RemoteC2) start(t *testing.T, stage string) {
	t.Helper()
	environment := remote.environment
	ports := fmt.Sprintf(":(%d|%d|%d|%d|%d|%d|%d|%d)[[:space:]]", environment.port, environment.port+1, environment.port+2,
		environment.port+3, environment.port+4, environment.port+5, environment.port+6, environment.port+7)
	preflight := fmt.Sprintf(`set -eu
if [ -e %[1]s ]; then printf 'H4-3B multi-host remote directory already exists\n' >&2; exit 1; fi
if docker container inspect %[2]s >/dev/null 2>&1; then printf 'H4-3B multi-host generated container already exists\n' >&2; exit 1; fi
if ! docker image inspect %[3]s >/dev/null; then printf 'H4-3B multi-host required Docker image is unavailable\n' >&2; exit 1; fi
if ss -ltnH | grep -E %[4]s >/dev/null; then printf 'H4-3B multi-host selected port is already listening\n' >&2; exit 1; fi`,
		h43ShellQuote(environment.remoteDirectory), h43ShellQuote(environment.container), h43ShellQuote(h43RemoteImage), h43ShellQuote(ports))
	if output, err := remote.run(t, preflight); err != nil {
		t.Fatalf("H4-3B multi-host remote environment is unavailable: %v\n%s", err, output)
	}
	if err := remote.upload(t, stage); err != nil {
		t.Fatalf("upload bounded H4-3B multi-host bundle: %v", err)
	}
	command := fmt.Sprintf("set -eu; docker run --detach --name %s --network host --pids-limit 128 --memory 1g --cpus 1 -v %s:/work --workdir /work %s /bin/sh /work/run.sh",
		h43ShellQuote(environment.container), h43ShellQuote(environment.remoteDirectory), h43ShellQuote(h43RemoteImage))
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("start remote H4-3B C-2 roles: %v\n%s", err, output)
	}
}

func (remote h43RemoteC2) waitFile(t *testing.T, path string, deadline time.Time) {
	t.Helper()
	for time.Now().Before(deadline) {
		if _, err := remote.readFile(t, path); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("remote H4-3B output %q did not become available\n%s", path, remote.logs(t))
}

func (remote h43RemoteC2) readFile(t *testing.T, path string) ([]byte, error) {
	t.Helper()
	return remote.readFileContext(t.Context(), path)
}

func (remote h43RemoteC2) readFileContext(ctx context.Context, path string) ([]byte, error) {
	if !strings.HasPrefix(path, remote.environment.remoteDirectory+"/") {
		return nil, errors.New("remote H4-3B output path escaped its generated directory")
	}
	output, err := remote.commandContext(ctx, "set -eu; test -s "+h43ShellQuote(path)+"; cat "+h43ShellQuote(path)).CombinedOutput()
	return []byte(output), err
}

func (remote h43RemoteC2) copyFile(t *testing.T, source, destination string, deadline time.Time) {
	t.Helper()
	remote.waitFile(t, source, deadline)
	contents, err := remote.readFile(t, source)
	if err != nil || len(contents) == 0 {
		t.Fatalf("read remote H4-3B output %q: %v", source, err)
	}
	if err := os.WriteFile(destination, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (remote h43RemoteC2) complete(t *testing.T) {
	t.Helper()
	path := remote.environment.remoteDirectory + "/complete"
	if output, err := remote.run(t, "set -eu; test ! -e "+h43ShellQuote(path)+"; printf 'complete\\n' > "+h43ShellQuote(path)); err != nil {
		t.Fatalf("release remote H4-3B roles: %v\n%s", err, output)
	}
}

func (remote h43RemoteC2) wait(t *testing.T) {
	t.Helper()
	output, err := remote.run(t, "docker wait "+h43ShellQuote(remote.environment.container))
	if err != nil || strings.TrimSpace(output) != "0" {
		t.Fatalf("remote H4-3B roles did not complete successfully: %v / %s\n%s", err, output, remote.logs(t))
	}
}

func (remote h43RemoteC2) hostEnvelope(t *testing.T) string {
	t.Helper()
	output, err := remote.run(t, "set -eu; printf 'docker='; docker version --format '{{.Server.Version}}'; printf 'image_id='; docker image inspect "+h43ShellQuote(h43RemoteImage)+" --format '{{.Id}}'; printf 'kernel='; uname -srmo; printf 'vcpus='; nproc; awk '/MemTotal:/ {printf \"memory_kib=%s\\n\", $2}' /proc/meminfo")
	if err != nil {
		t.Fatalf("read H4-3B remote host envelope: %v\n%s", err, output)
	}
	return strings.Join(strings.Fields(output), " ")
}

func (remote h43RemoteC2) logs(t *testing.T) string {
	t.Helper()
	directory := h43ShellQuote(remote.environment.remoteDirectory)
	output, err := remote.run(t, "set -eu; docker logs "+h43ShellQuote(remote.environment.container)+" 2>&1 || true; for file in "+directory+"/*.log "+directory+"/*.err; do if [ -f \"$file\" ]; then echo ===$(basename \"$file\")===; cat \"$file\"; fi; done")
	if err != nil {
		return fmt.Sprintf("remote H4-3B logs unavailable: %v\n%s", err, output)
	}
	return output
}

func (remote h43RemoteC2) remove(t *testing.T) {
	t.Helper()
	environment := remote.environment
	command := fmt.Sprintf("set -eu; docker rm -f %s >/dev/null 2>&1 || true; if [ -e %s ]; then rm -rf -- %s; fi; test ! -e %s; ! docker container inspect %s >/dev/null 2>&1",
		h43ShellQuote(environment.container), h43ShellQuote(environment.remoteDirectory), h43ShellQuote(environment.remoteDirectory),
		h43ShellQuote(environment.remoteDirectory), h43ShellQuote(environment.container))
	if output, err := remote.runCleanup(command); err != nil {
		t.Errorf("remove H4-3B multi-host remote resources: %v\n%s", err, output)
	}
}

func (remote h43RemoteC2) upload(t *testing.T, stage string) error {
	t.Helper()
	environment := remote.environment
	command := fmt.Sprintf("set -eu; test ! -e %s; mkdir -m 700 %s; tar -xzf - -C %s; chmod 700 %s/reference-c2 %s/ardents-node %s/run.sh",
		h43ShellQuote(environment.remoteDirectory), h43ShellQuote(environment.remoteDirectory), h43ShellQuote(environment.remoteDirectory),
		h43ShellQuote(environment.remoteDirectory), h43ShellQuote(environment.remoteDirectory), h43ShellQuote(environment.remoteDirectory))
	process := remote.command(t, command)
	stdin, err := process.StdinPipe()
	if err != nil {
		return err
	}
	var output bytes.Buffer
	process.Stdout, process.Stderr = &output, &output
	if err := process.Start(); err != nil {
		return err
	}
	archiveErr := h43WriteArchive(stage, stdin)
	closeErr := stdin.Close()
	waitErr := process.Wait()
	if archiveErr != nil || closeErr != nil || waitErr != nil {
		return fmt.Errorf("remote archive transfer: archive=%v close=%v remote=%v\n%s", archiveErr, closeErr, waitErr, output.String())
	}
	return nil
}

func (remote h43RemoteC2) run(t *testing.T, command string) (string, error) {
	t.Helper()
	output, err := remote.command(t, command).CombinedOutput()
	return string(output), err
}

func (remote h43RemoteC2) command(t *testing.T, command string) *exec.Cmd {
	t.Helper()
	return remote.commandContext(t.Context(), command)
}

func (remote h43RemoteC2) runCleanup(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := remote.commandContext(ctx, command).CombinedOutput()
	return string(output), err
}

func (remote h43RemoteC2) commandContext(ctx context.Context, command string) *exec.Cmd {
	environment := remote.environment
	return exec.CommandContext(ctx, environment.sshPath, "-i", environment.sshKey, "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile="+environment.knownHosts,
		environment.user+"@"+environment.host, "sh -lc "+h43ShellQuote(command))
}

func h43WriteArchive(root string, writer io.Writer) error {
	gzipWriter := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gzipWriter)
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("refuse non-regular remote bundle input %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if walkErr != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return walkErr
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	return gzipWriter.Close()
}

func h43ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func h43RemoteRunner() string {
	return `#!/bin/sh
set -eu
work=/work
pids=""
transit_pids=""
started_pid=""
terminal=$(cat "$work/expected-terminal")
cleanup() {
  status=$?
  trap - EXIT INT TERM
  for pid in $pids; do kill "$pid" 2>/dev/null || true; done
  for pid in $pids; do wait "$pid" 2>/dev/null || true; done
  exit "$status"
}
trap cleanup EXIT INT TERM
start() {
  name=$1
  shift
  "$@" >"$work/$name.log" 2>"$work/$name.err" &
  pid=$!
  pids="$pids $pid"
  started_pid=$pid
}
wait_file() {
  path=$1
  tries=0
  while [ "$tries" -lt 400 ]; do
    if [ -s "$path" ]; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
wait_source() {
  log=$1
  tries=0
  while [ "$tries" -lt 200 ]; do
    if grep -F '"kind":"source-ready"' "$log" >/dev/null 2>&1; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
start source-a "$work/ardents-node" source --config "$work/source-a-plan.json"
start source-b "$work/ardents-node" source --config "$work/source-b-plan.json"
wait_source "$work/source-a.log"
wait_source "$work/source-b.log"
for role in rendezvous initiator introduction responder; do
  start "$role" "$work/reference-c2" "$role" "$work/reference-c2.json"
  transit_pids="$transit_pids $started_pid"
done
for role in rendezvous initiator introduction responder; do wait_file "$work/ready/$role"; done
start gateway "$work/reference-c2" gateway "$work/reference-c2.json"
gateway=$started_pid
start publisher "$work/reference-c2" publisher "$work/reference-c2.json"
publisher=$started_pid
wait_file "$work/publication.json"
start alpha-gateway "$work/reference-c2" alpha-gateway "$work/reference-c2.json"
alpha_gateway=$started_pid
wait_file "$work/alpha-gateway-ready.json"
start alpha-relay "$work/reference-c2" alpha-relay "$work/reference-c2.json"
alpha_relay=$started_pid
wait_file "$work/alpha-relay-ready.json"
wait_file "$work/ready/gateway"
start publisher-app "$work/reference-c2" publisher-app "$work/reference-c2.json"
publisher_app=$started_pid
if [ "$terminal" = endpoint-stop ]; then
  (wait_file "$work/publisher-crash-ready"; kill "$publisher" 2>/dev/null || true) &
  fault_pid=$!
  pids="$pids $fault_pid"
else
  fault_pid=""
fi
wait_file "$work/complete"
wait_expected() {
  name=$1
  pid=$2
  if wait "$pid"; then return 0; fi
  case "$terminal:$name" in
    application-reset:publisher-app|endpoint-stop:publisher|endpoint-stop:publisher-app) return 0 ;;
  esac
  return 1
}
for item in "publisher:$publisher" "publisher-app:$publisher_app" "gateway:$gateway" "alpha-gateway:$alpha_gateway" "alpha-relay:$alpha_relay"; do
  name=${item%%:*}
  pid=${item#*:}
  wait_expected "$name" "$pid"
done
if [ -n "$fault_pid" ]; then wait "$fault_pid"; fi
for pid in $transit_pids; do wait "$pid"; done
`
}

func TestH43RemoteRunnerHasPOSIXShellSyntax(t *testing.T) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal("Docker is unavailable for H4-3B remote-runner syntax validation")
	}
	command := exec.Command(docker, "run", "--rm", "-i", h43RemoteImage, "/bin/sh", "-n")
	command.Stdin = strings.NewReader(h43RemoteRunner())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("H4-3B remote runner shell syntax: %v\n%s", err, output)
	}
}

func TestH43WriteArchiveCarriesOnlyRelativeRegularInputs(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "input"), []byte("bounded"), 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := h43WriteArchive(root, &archive); err != nil {
		t.Fatal(err)
	}
	compressed, err := gzip.NewReader(&archive)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var names []string
	for {
		header, readErr := reader.Next()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		if filepath.IsAbs(header.Name) || strings.HasPrefix(header.Name, "../") || header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			t.Fatalf("remote archive entry is outside the bounded regular-file contract: %+v", header)
		}
		names = append(names, header.Name)
	}
	if strings.Join(names, ",") != "nested,nested/input" {
		t.Fatalf("remote archive entries = %v", names)
	}
}
