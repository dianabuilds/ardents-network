package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type serviceProcess struct {
	command *exec.Cmd
	decoder *json.Decoder
	stderr  bytes.Buffer
}

func startServiceProcess(t *testing.T, ctx context.Context, binary, root, plan string) *serviceProcess {
	t.Helper()
	process := &serviceProcess{command: exec.CommandContext(ctx, binary, "endpoint", "run", plan)}
	process.command.Dir = root
	stdout, err := process.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process.command.Stderr = &process.stderr
	process.decoder = json.NewDecoder(stdout)
	if err := process.command.Start(); err != nil {
		t.Fatal(err)
	}
	var ready map[string]string
	if err := process.decoder.Decode(&ready); err != nil || ready["kind"] != "ready" {
		_ = process.command.Process.Kill()
		t.Fatalf("Service did not become ready: event=%v err=%v stderr=%s", ready, err, process.stderr.String())
	}
	t.Cleanup(func() {
		if process.command.ProcessState == nil {
			_ = process.command.Process.Kill()
			_, _ = process.command.Process.Wait()
		}
	})
	return process
}

func (process *serviceProcess) finish(t *testing.T, output any) {
	t.Helper()
	if err := process.decoder.Decode(output); err != nil {
		_ = process.command.Process.Kill()
		t.Fatalf("decode Service result: %v stderr=%s", err, process.stderr.String())
	}
	if err := process.command.Wait(); err != nil {
		t.Fatalf("Service process failed: %v stderr=%s", err, process.stderr.String())
	}
}

func runCommand(t *testing.T, ctx context.Context, root, binary string, arguments ...string) []byte {
	t.Helper()
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, output)
	}
	return output
}

func buildProductCommand(t *testing.T, name string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	filename := name
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	path := filepath.Join(t.TempDir(), filename)
	command := exec.Command("go", "build", "-trimpath", "-o", path, "./cmd/"+name)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return path
}

func buildE2EFixtureCommand(t *testing.T, name string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	filename := name
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	path := filepath.Join(t.TempDir(), filename)
	command := exec.Command("go", "build", "-trimpath", "-o", path, "./tests/e2e/service/fixturecommand/"+name)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build e2e fixture %s: %v\n%s", name, err, output)
	}
	return path
}

type replacementRouteObservation struct {
	firstInterruption, secondInterruption, finished time.Time
	carrierBytes                                    int64
}

func runReplacementRoute(ctx context.Context, clientPath, publisherPath string) (replacementRouteObservation, error) {
	var observation replacementRouteObservation
	client, publisher, err := dialRoutePair(ctx, clientPath, publisherPath)
	if err != nil {
		return observation, err
	}
	count, err := bridgeRoute(ctx, client, publisher, 512<<10, func() { observation.firstInterruption = time.Now() })
	observation.carrierBytes += count
	if err != nil {
		return observation, err
	}
	failedClient, failedPublisher, err := dialRoutePair(ctx, clientPath, publisherPath)
	if err != nil {
		return observation, err
	}
	observation.secondInterruption = time.Now()
	_ = failedClient.Close()
	_ = failedPublisher.Close()
	client, publisher, err = dialRoutePair(ctx, clientPath, publisherPath)
	if err != nil {
		return observation, err
	}
	count, err = bridgeRoute(ctx, client, publisher, 0, nil)
	observation.carrierBytes += count
	observation.finished = time.Now()
	return observation, err
}

func dialRoutePair(ctx context.Context, clientPath, publisherPath string) (net.Conn, net.Conn, error) {
	client, err := dialUnixUntil(ctx, clientPath)
	if err != nil {
		return nil, nil, err
	}
	publisher, err := dialUnixUntil(ctx, publisherPath)
	if err != nil {
		client.Close()
		return nil, nil, err
	}
	return client, publisher, nil
}

func dialUnixUntil(ctx context.Context, path string) (net.Conn, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		connection, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
		if err == nil {
			return connection, nil
		}
		select {
		case <-ctx.Done():
			return nil, errors.Join(ctx.Err(), err)
		case <-ticker.C:
		}
	}
}

func bridgeRoute(ctx context.Context, left, right net.Conn, byteLimit int64, interrupt func()) (int64, error) {
	defer left.Close()
	defer right.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = left.Close()
		_ = right.Close()
	})
	defer stop()
	type copyResult struct {
		count int64
		err   error
	}
	results := make(chan copyResult, 2)
	copyDirection := func(destination, source net.Conn) {
		var reader io.Reader = source
		if byteLimit > 0 {
			reader = io.LimitReader(source, byteLimit)
		}
		count, err := io.Copy(destination, reader)
		results <- copyResult{count: count, err: err}
	}
	go copyDirection(right, left)
	go copyDirection(left, right)
	first := <-results
	if byteLimit > 0 && first.count >= byteLimit {
		if interrupt != nil {
			interrupt()
		}
		_ = left.Close()
		_ = right.Close()
		second := <-results
		return first.count + second.count, nil
	}
	_ = left.Close()
	_ = right.Close()
	second := <-results
	if ctx.Err() != nil {
		return first.count + second.count, ctx.Err()
	}
	return first.count + second.count, benignBridgeError(first.err, second.err)
}

func benignBridgeError(values ...error) error {
	for _, err := range values {
		if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
			return err
		}
	}
	return nil
}

type commandResult struct {
	output []byte
	err    error
}

func startCommand(ctx context.Context, root, binary string, arguments ...string) <-chan commandResult {
	result := make(chan commandResult, 1)
	go func() {
		command := exec.CommandContext(ctx, binary, arguments...)
		command.Dir = root
		output, err := command.CombinedOutput()
		result <- commandResult{output: output, err: err}
	}()
	return result
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
