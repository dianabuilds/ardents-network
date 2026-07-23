package ingressproxy

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConnectionResetDoesNotStopIngressListener(t *testing.T) {
	backend := listenTCP(t)
	ingress := listenTCP(t)
	backendPort := uint16(backend.Addr().(*net.TCPAddr).Port)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() {
		result <- runWithListeners(ctx, DefaultConfig("127.0.0.1", []uint16{backendPort}),
			[]listenerBinding{{listener: ingress, targetPort: backendPort}})
	}()

	firstBackend := make(chan net.Conn, 1)
	go func() {
		connection, err := backend.Accept()
		if err == nil {
			firstBackend <- connection
		}
	}()
	firstClient, err := net.DialTimeout("tcp", ingress.Addr().String(), time.Second)
	require.NoError(t, err)
	reset := <-firstBackend
	require.NoError(t, reset.(*net.TCPConn).SetLinger(0))
	require.NoError(t, reset.Close())
	_, _ = firstClient.Write([]byte("trigger reset"))
	_ = firstClient.SetReadDeadline(time.Now().Add(time.Second))
	_, _ = firstClient.Read(make([]byte, 1))
	require.NoError(t, firstClient.Close())

	go func() {
		connection, acceptErr := backend.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		_, _ = io.Copy(connection, connection)
	}()
	secondClient, err := net.DialTimeout("tcp", ingress.Addr().String(), time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = secondClient.Close() })
	require.NoError(t, secondClient.SetDeadline(time.Now().Add(2*time.Second)))
	_, err = secondClient.Write([]byte("listener survived"))
	require.NoError(t, err)
	response := make([]byte, len("listener survived"))
	_, err = io.ReadFull(secondClient, response)
	require.NoError(t, err)
	require.Equal(t, "listener survived", string(response))

	select {
	case err := <-result:
		t.Fatalf("proxy stopped after connection reset: %v", err)
	default:
	}
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
}

func TestHalfCloseCompletesResponseAndKeepsListenerAvailable(t *testing.T) {
	backend := listenTCP(t)
	ingress := listenTCP(t)
	backendPort := uint16(backend.Addr().(*net.TCPAddr).Port)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() {
		result <- runWithListeners(ctx, DefaultConfig("127.0.0.1", []uint16{backendPort}),
			[]listenerBinding{{listener: ingress, targetPort: backendPort}})
	}()
	go func() {
		connection, err := backend.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = io.ReadAll(connection)
		_, _ = connection.Write([]byte("after-half-close"))
	}()

	client, err := net.DialTCP("tcp", nil, ingress.Addr().(*net.TCPAddr))
	require.NoError(t, err)
	require.NoError(t, client.SetDeadline(time.Now().Add(2*time.Second)))
	_, err = client.Write([]byte("request"))
	require.NoError(t, err)
	require.NoError(t, client.CloseWrite())
	response, err := io.ReadAll(client)
	require.NoError(t, err)
	require.Equal(t, "after-half-close", string(response))
	require.NoError(t, client.Close())

	echoAcceptedConnections(t, backend)
	replacement, err := net.DialTimeout("tcp", ingress.Addr().String(), time.Second)
	require.NoError(t, err)
	require.NoError(t, replacement.SetDeadline(time.Now().Add(2*time.Second)))
	_, err = replacement.Write([]byte("replacement"))
	require.NoError(t, err)
	echo := make([]byte, len("replacement"))
	_, err = io.ReadFull(replacement, echo)
	require.NoError(t, err)
	require.NoError(t, replacement.Close())
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
}

func TestIdleSourceCannotExhaustIndependentIngress(t *testing.T) {
	idleBackend := listenTCP(t)
	echoBackend := listenTCP(t)
	idleIngress := listenTCP(t)
	echoIngress := listenTCP(t)
	var (
		eventsMu sync.Mutex
		events   []Event
	)
	config := DefaultConfig("127.0.0.1", []uint16{
		uint16(idleBackend.Addr().(*net.TCPAddr).Port),
		uint16(echoBackend.Addr().(*net.TCPAddr).Port),
	})
	config.IdleTimeout = 5 * time.Second
	config.Observe = func(event Event) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() {
		result <- runWithListeners(ctx, config, []listenerBinding{
			{listener: idleIngress, targetPort: config.Ports[0]},
			{listener: echoIngress, targetPort: config.Ports[1]},
		})
	}()
	holdAcceptedConnections(t, idleBackend)
	echoAcceptedConnections(t, echoBackend)

	idleClients := make([]net.Conn, 0, 128)
	for range 128 {
		client, err := (&net.Dialer{
			LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.2")},
			Timeout:   time.Second,
		}).Dial("tcp", idleIngress.Addr().String())
		require.NoError(t, err)
		idleClients = append(idleClients, client)
	}
	t.Cleanup(func() {
		for _, client := range idleClients {
			_ = client.Close()
		}
	})

	legitimate, err := (&net.Dialer{
		LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.3")},
		Timeout:   time.Second,
	}).Dial("tcp", echoIngress.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = legitimate.Close() })
	require.NoError(t, legitimate.SetDeadline(time.Now().Add(2*time.Second)))
	_, err = legitimate.Write([]byte("independent"))
	require.NoError(t, err)
	response := make([]byte, len("independent"))
	_, err = io.ReadFull(legitimate, response)
	require.NoError(t, err)
	require.Equal(t, "independent", string(response))

	require.Eventually(t, func() bool {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		for _, event := range events {
			if event.Type == EventConnectionRejected && event.Reason == ReasonSourceLimit {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
}

func TestIdleConnectionExpiresAndListenerAcceptsReplacement(t *testing.T) {
	backend := listenTCP(t)
	ingress := listenTCP(t)
	backendPort := uint16(backend.Addr().(*net.TCPAddr).Port)
	config := DefaultConfig("127.0.0.1", []uint16{backendPort})
	config.IdleTimeout = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() {
		result <- runWithListeners(ctx, config,
			[]listenerBinding{{listener: ingress, targetPort: backendPort}})
	}()
	holdAcceptedConnections(t, backend)

	idle, err := net.DialTimeout("tcp", ingress.Addr().String(), time.Second)
	require.NoError(t, err)
	require.NoError(t, idle.SetReadDeadline(time.Now().Add(time.Second)))
	_, err = idle.Read(make([]byte, 1))
	require.Error(t, err)
	require.NoError(t, idle.Close())

	replacement, err := net.DialTimeout("tcp", ingress.Addr().String(), time.Second)
	require.NoError(t, err)
	require.NoError(t, replacement.Close())
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
}

func TestSustainedOneWayTrafficIsNotIdle(t *testing.T) {
	backend := listenTCP(t)
	ingress := listenTCP(t)
	backendPort := uint16(backend.Addr().(*net.TCPAddr).Port)
	config := DefaultConfig("127.0.0.1", []uint16{backendPort})
	config.IdleTimeout = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() {
		result <- runWithListeners(ctx, config,
			[]listenerBinding{{listener: ingress, targetPort: backendPort}})
	}()

	received := make(chan int64, 1)
	go func() {
		connection, err := backend.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		count, _ := io.Copy(io.Discard, connection)
		received <- count
	}()

	client, err := net.DialTimeout("tcp", ingress.Addr().String(), time.Second)
	require.NoError(t, err)
	require.NoError(t, client.SetWriteDeadline(time.Now().Add(time.Second)))
	const writes = 12
	for range writes {
		_, err = client.Write([]byte("active"))
		require.NoError(t, err)
		time.Sleep(25 * time.Millisecond)
	}
	require.NoError(t, client.Close())
	require.Equal(t, int64(writes*len("active")), <-received)

	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
}

func TestPortAdmissionLeavesCapacityForAnotherPort(t *testing.T) {
	firstBackend := listenTCP(t)
	secondBackend := listenTCP(t)
	firstIngress := listenTCP(t)
	secondIngress := listenTCP(t)
	config := DefaultConfig("127.0.0.1", []uint16{
		uint16(firstBackend.Addr().(*net.TCPAddr).Port),
		uint16(secondBackend.Addr().(*net.TCPAddr).Port),
	})
	config.MaxConnections = 4
	config.MaxConnectionsPerPort = 2
	config.MaxConnectionsPerSource = 4
	config.IdleTimeout = 5 * time.Second
	var (
		eventsMu sync.Mutex
		events   []Event
	)
	config.Observe = func(event Event) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- runWithListeners(ctx, config, []listenerBinding{
			{listener: firstIngress, targetPort: config.Ports[0]},
			{listener: secondIngress, targetPort: config.Ports[1]},
		})
	}()
	holdAcceptedConnections(t, firstBackend)
	echoAcceptedConnections(t, secondBackend)

	var clients []net.Conn
	for index, source := range []string{"127.0.0.2", "127.0.0.3", "127.0.0.4"} {
		client, err := (&net.Dialer{
			LocalAddr: &net.TCPAddr{IP: net.ParseIP(source), Port: 41000 + index},
			Timeout:   time.Second,
		}).Dial("tcp", firstIngress.Addr().String())
		require.NoError(t, err)
		clients = append(clients, client)
	}
	t.Cleanup(func() {
		for _, client := range clients {
			_ = client.Close()
		}
	})
	require.Eventually(t, func() bool {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		for _, event := range events {
			if event.Type == EventConnectionRejected && event.Reason == ReasonPortLimit {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	legitimate, err := (&net.Dialer{
		LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.5")},
		Timeout:   time.Second,
	}).Dial("tcp", secondIngress.Addr().String())
	require.NoError(t, err)
	require.NoError(t, legitimate.SetDeadline(time.Now().Add(2*time.Second)))
	_, err = legitimate.Write([]byte("fair"))
	require.NoError(t, err)
	reply := make([]byte, len("fair"))
	_, err = io.ReadFull(legitimate, reply)
	require.NoError(t, err)
	require.NoError(t, legitimate.Close())
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
}

func TestConfigRejectsUnsafeDeadlineAndAdmissionCombinations(t *testing.T) {
	config := DefaultConfig("workload", []uint16{19000})
	require.Equal(t, 128, config.MaxConnections)
	require.Equal(t, 64, config.MaxConnectionsPerPort)
	require.Equal(t, 16, config.MaxConnectionsPerSource)
	require.Equal(t, 30*time.Second, config.IdleTimeout)

	config.IdleTimeout = 0
	require.ErrorContains(t, RunConfig(context.Background(), config), "deadlines")
	config = DefaultConfig("workload", []uint16{19000})
	config.MaxConnectionsPerPort = config.MaxConnections + 1
	require.ErrorContains(t, RunConfig(context.Background(), config), "connection limits")
}

func listenTCP(t *testing.T) *net.TCPListener {
	t.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func holdAcceptedConnections(t *testing.T, listener net.Listener) {
	t.Helper()
	var (
		mu          sync.Mutex
		connections []net.Conn
	)
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, connection := range connections {
			_ = connection.Close()
		}
	})
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			connections = append(connections, connection)
			mu.Unlock()
		}
	}()
}

func echoAcceptedConnections(t *testing.T, listener net.Listener) {
	t.Helper()
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer connection.Close()
				_, _ = io.Copy(connection, connection)
			}()
		}
	}()
}
