//go:build !windows

package daemon

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	networkapi "ardents/internal/network"

	"github.com/stretchr/testify/require"
)

func TestProcessLevelActiveStreamShutdown(t *testing.T) {
	if os.Getenv("ARDENTS_SHUTDOWN_STREAM_HELPER") == "1" {
		runActiveStreamShutdownHelper()
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestProcessLevelActiveStreamShutdown$")
	command.Env = append(os.Environ(), "ARDENTS_SHUTDOWN_STREAM_HELPER=1")
	stdout, err := command.StdoutPipe()
	require.NoError(t, err)
	command.Stderr = os.Stderr
	require.NoError(t, command.Start())

	lines := make(chan string, 4)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	address := requireHelperLine(t, lines, "LISTEN ")
	response := make(chan *http.Response, 1)
	go func() {
		result, requestErr := http.Get("http://" + address)
		if requestErr == nil {
			response <- result
		}
	}()
	require.Equal(t, "STREAM", requireHelperLine(t, lines, ""))
	require.NoError(t, command.Process.Signal(syscall.SIGTERM))

	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	select {
	case waitErr := <-exited:
		require.NoError(t, waitErr)
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("daemon helper exceeded the process shutdown budget")
	}
	select {
	case result := <-response:
		_ = result.Body.Close()
	default:
	}
	require.Equal(t, "CLEANUP", requireHelperLine(t, lines, ""))
}

func requireHelperLine(t *testing.T, lines <-chan string, trimPrefix string) string {
	t.Helper()
	select {
	case line, ok := <-lines:
		require.True(t, ok, "shutdown helper closed output unexpectedly")
		if trimPrefix == "" {
			return line
		}
		require.Contains(t, line, trimPrefix)
		return line[len(trimPrefix):]
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown helper did not reach its lifecycle checkpoint")
		return ""
	}
}

func runActiveStreamShutdownHelper() {
	dataDir, err := os.MkdirTemp("", "ardents-shutdown-process-")
	if err != nil {
		os.Exit(2)
	}
	defer os.RemoveAll(dataDir)
	node := NewNode(Config{
		Name: "shutdown-process", NodeProfile: networkapi.NodeProfileConstrainedClient,
		Data: DataConfig{Dir: dataDir},
	})
	startCtx, cancelStart := context.WithTimeout(context.Background(), 3*time.Second)
	if err := node.Start(startCtx); err != nil {
		cancelStart()
		os.Exit(2)
	}
	cancelStart()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	process, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	server := newHTTPServer(listener.Addr().String(), http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		fmt.Println("STREAM")
		<-request.Context().Done()
	}))
	fmt.Println("LISTEN " + listener.Addr().String())
	if err := serveDrainAndCleanup(process, stop, []*http.Server{server}, func() error {
		return stopNode(node)
	}, serveTarget{
		serve: func() error { return server.Serve(listener) },
	}); err != nil {
		os.Exit(3)
	}
	if node.GetNodeRuntime().Node.State != "stopped" {
		os.Exit(4)
	}
	fmt.Println("CLEANUP")
}
