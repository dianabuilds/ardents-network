package modulecache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const commandOutputLimit = 1 << 20

func moduleEnvironment(cache, proxy, sumdb string) []string {
	blocked := map[string]bool{"ALL_PROXY": true, "GO111MODULE": true, "GOAUTH": true,
		"GOCACHE": true, "GOENV": true, "GOFLAGS": true, "GOINSECURE": true,
		"GOMODCACHE": true, "GONOPROXY": true, "GONOSUMDB": true, "GOPRIVATE": true,
		"GOPROXY": true, "GOSUMDB": true, "GOTOOLCHAIN": true, "GOVCS": true, "GOWORK": true,
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true}
	environment := make([]string, 0, len(os.Environ())+14)
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if !blocked[strings.ToUpper(name)] {
			environment = append(environment, value)
		}
	}
	return append(environment, "GOCACHE="+cache+string(os.PathSeparator)+".gocache",
		"GOMODCACHE="+cache, "GOTOOLCHAIN=local", "GOWORK=off",
		"GO111MODULE=on", "GOFLAGS=-mod=readonly", "GOENV=off", "GOPROXY="+proxy,
		"GOSUMDB="+sumdb, "GONOSUMDB=", "GOPRIVATE=", "GONOPROXY=", "GOINSECURE=",
		"GOAUTH=off", "GOVCS=*:off", "ALL_PROXY=", "HTTP_PROXY=", "HTTPS_PROXY=",
		"NO_PROXY=proxy.golang.org,sum.golang.org")
}

func boundedGo(root string, environment []string, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", arguments...)
	command.Dir, command.Env = root, environment
	if err := prepareModuleProcess(command); err != nil {
		return nil, err
	}
	command.WaitDelay = 5 * time.Second
	command.Cancel = func() error { return terminateModuleProcess(command) }
	var stdout, stderr boundedBuffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if ctx.Err() != nil || stdout.overflow || stderr.overflow {
		return nil, errors.New("go module command exceeded its time or output bound")
	}
	if err != nil {
		return nil, fmt.Errorf("go %s: %w: %s", strings.Join(arguments, " "), err, stderr.String())
	}
	return stdout.Bytes(), nil
}

type boundedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (value *boundedBuffer) Write(input []byte) (int, error) {
	written := len(input)
	remaining := commandOutputLimit - value.Len()
	if remaining > 0 {
		_, _ = value.Buffer.Write(input[:min(remaining, len(input))])
	}
	value.overflow = value.overflow || written > remaining
	return written, nil
}

func (value *boundedBuffer) String() string { return strings.TrimSpace(value.Buffer.String()) }
