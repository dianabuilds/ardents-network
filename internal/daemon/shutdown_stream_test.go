package daemon

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServeAndDrainCancelsActiveStreamFromProcessContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	server := newHTTPServer(listener.Addr().String(), http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(entered)
		writer.WriteHeader(http.StatusOK)
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		<-request.Context().Done()
		close(cancelled)
	}))
	process, signal := context.WithCancel(context.Background())
	drained := make(chan error, 1)
	go func() {
		drained <- serveAndDrain(process, signal, []*http.Server{server}, serveTarget{
			serve: func() error { return server.Serve(listener) },
		})
	}()
	response, err := http.Get("http://" + listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	<-entered

	signal()
	deadline, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-cancelled:
	case <-deadline.Done():
		_ = server.Close()
		t.Fatal("active stream did not receive process cancellation")
	}
	select {
	case err := <-drained:
		require.NoError(t, err)
	case <-deadline.Done():
		_ = server.Close()
		t.Fatal("server drain exceeded its bounded test deadline")
	}
}

func TestServeAndDrainRejectsNewStreamsAndForcesBoundedTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	listenerClosed := make(chan struct{})
	server := newHTTPServer(listener.Addr().String(), http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(entered)
		writer.WriteHeader(http.StatusOK)
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		<-request.Context().Done()
		close(cancelled)
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		_ = server.Close()
	})
	process, signal := context.WithCancel(context.Background())
	drained := make(chan error, 1)
	go func() {
		drained <- serveAndDrainWithBudget(process, signal, 50*time.Millisecond, []*http.Server{server}, serveTarget{
			serve: func() error {
				err := server.Serve(listener)
				close(listenerClosed)
				return err
			},
		})
	}()
	response, err := http.Get("http://" + listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	<-entered

	signal()
	<-cancelled
	<-listenerClosed
	_, err = http.Get("http://" + listener.Addr().String())
	require.Error(t, err, "new stream was accepted after drain began")

	deadline, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case err := <-drained:
		require.ErrorContains(t, err, "API drain deadline exceeded")
	case <-deadline.Done():
		t.Fatal("forced API drain exceeded its bounded test deadline")
	}
}
