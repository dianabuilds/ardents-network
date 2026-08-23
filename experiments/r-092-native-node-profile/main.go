//go:build ignore

// R-092 synthetic preannouncement baseline. See README.md and the research record.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const profile = "ardents-interactive-route-v1"

type measurement struct {
	Schema              string            `json:"schema"`
	Connections         int               `json:"connections"`
	PayloadBytes        int               `json:"payload_bytes"`
	TotalCarriedBytes   int               `json:"total_carried_bytes"`
	ElapsedNanoseconds  int64             `json:"elapsed_nanoseconds"`
	AllocatedBytes      uint64            `json:"allocated_bytes"`
	HeapAllocationDelta int64             `json:"heap_allocation_delta"`
	GoroutinesBefore    int               `json:"goroutines_before"`
	GoroutinesAfter     int               `json:"goroutines_after"`
	Linux               *linuxMeasurement `json:"linux,omitempty"`
}

type linuxMeasurement struct {
	Before linuxSample `json:"before"`
	After  linuxSample `json:"after"`
}

type linuxSample struct {
	UnixNano, RSSBytes, UserTicks, SystemTicks int64
	FileDescriptors                            int
}

func main() {
	connections := flag.Int("connections", 1, "synthetic sequential mTLS legs (1..32)")
	payloadSize := flag.Int("payload", 64<<10, "opaque bytes per leg (1..1048576)")
	timeout := flag.Duration("timeout", 10*time.Second, "complete baseline deadline")
	flag.Parse()
	if *connections < 1 || *connections > 32 || *payloadSize < 1 || *payloadSize > 1<<20 || *timeout <= 0 {
		fail(errors.New("baseline flags are outside their bound"))
	}
	result, err := run(*connections, *payloadSize, *timeout)
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
}

func run(connections, payloadSize int, timeout time.Duration) (measurement, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, clientKey, err := identity(1)
	if err != nil {
		return measurement{}, err
	}
	server, serverKey, err := identity(2)
	if err != nil {
		return measurement{}, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return measurement{}, err
	}
	defer listener.Close()
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	payload := make([]byte, payloadSize)
	if _, err := rand.Read(payload); err != nil {
		return measurement{}, err
	}
	notAfter := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	serverDone := make(chan error, 1)
	go serve(ctx, listener, server, clientKey, connections, payload, notAfter, serverDone)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	linuxBefore, err := sampleLinux()
	if err != nil {
		return measurement{}, err
	}
	started, goroutines := time.Now(), runtime.NumGoroutine()
	for index := 0; index < connections; index++ {
		if err := carry(ctx, listener.Addr().String(), client, serverKey, index, payload, notAfter); err != nil {
			return measurement{}, err
		}
	}
	if err := <-serverDone; err != nil {
		return measurement{}, err
	}
	runtime.ReadMemStats(&after)
	linuxAfter, err := sampleLinux()
	if err != nil {
		return measurement{}, err
	}
	result := measurement{Schema: "ardents-r092-native-leg-baseline-v1", Connections: connections,
		PayloadBytes: payloadSize, TotalCarriedBytes: connections * payloadSize * 2,
		ElapsedNanoseconds: time.Since(started).Nanoseconds(), AllocatedBytes: after.TotalAlloc - before.TotalAlloc,
		HeapAllocationDelta: int64(after.HeapAlloc) - int64(before.HeapAlloc), GoroutinesBefore: goroutines,
		GoroutinesAfter: runtime.NumGoroutine()}
	if linuxBefore != nil && linuxAfter != nil {
		result.Linux = &linuxMeasurement{Before: *linuxBefore, After: *linuxAfter}
	}
	return result, nil
}

func sampleLinux() (*linuxSample, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return nil, err
	}
	var rss int64
	for _, line := range strings.Split(string(status), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, errors.New("Linux RSS observation is invalid")
		}
		value, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil || value < 0 {
			return nil, errors.New("Linux RSS observation is invalid")
		}
		rss = value * 1024
	}
	if rss == 0 {
		return nil, errors.New("Linux RSS observation is absent")
	}
	stat, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return nil, err
	}
	right := strings.LastIndex(string(stat), ")")
	if right < 0 {
		return nil, errors.New("Linux CPU observation is invalid")
	}
	fields := strings.Fields(string(stat)[right+1:])
	if len(fields) < 13 {
		return nil, errors.New("Linux CPU observation is invalid")
	}
	user, userErr := strconv.ParseInt(fields[11], 10, 64)
	system, systemErr := strconv.ParseInt(fields[12], 10, 64)
	if userErr != nil || systemErr != nil || user < 0 || system < 0 {
		return nil, errors.New("Linux CPU observation is invalid")
	}
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return nil, err
	}
	return &linuxSample{UnixNano: time.Now().UnixNano(), RSSBytes: rss, UserTicks: user,
		SystemTicks: system, FileDescriptors: len(fds)}, nil
}

func serve(ctx context.Context, listener net.Listener, certificate tls.Certificate, client ed25519.PublicKey,
	connections int, payload []byte, notAfter time.Time, done chan<- error,
) {
	for index := 0; index < connections; index++ {
		connection, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		secured := tls.Server(connection, config(certificate, client, true))
		if err := secured.HandshakeContext(ctx); err != nil {
			_ = secured.Close()
			done <- err
			return
		}
		if err := route.AcceptNodeLegBinding(secured, binding(index, false, notAfter)); err != nil {
			_ = secured.Close()
			done <- err
			return
		}
		got := make([]byte, len(payload))
		if _, err := io.ReadFull(secured, got); err != nil || !bytes.Equal(got, payload) {
			_ = secured.Close()
			done <- errors.New("synthetic opaque payload is invalid")
			return
		}
		if err := writeAll(secured, got); err != nil {
			_ = secured.Close()
			done <- err
			return
		}
		if err := secured.Close(); err != nil {
			done <- err
			return
		}
	}
	done <- nil
}

func carry(ctx context.Context, address string, certificate tls.Certificate, server ed25519.PublicKey,
	index int, payload []byte, notAfter time.Time,
) error {
	dialer := &net.Dialer{}
	raw, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	secured := tls.Client(raw, config(certificate, server, false))
	defer secured.Close()
	if err := secured.HandshakeContext(ctx); err != nil {
		return err
	}
	if err := route.ConfirmNodeLegBinding(secured, binding(index, true, notAfter)); err != nil {
		return err
	}
	if err := writeAll(secured, payload); err != nil {
		return err
	}
	echo := make([]byte, len(payload))
	if _, err := io.ReadFull(secured, echo); err != nil || !bytes.Equal(echo, payload) {
		return errors.New("synthetic opaque payload echo is invalid")
	}
	return nil
}

func identity(serial int64) (tls.Certificate, ed25519.PublicKey, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	certificate := x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, &certificate, &certificate, public, private)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, public, nil
}

func config(certificate tls.Certificate, expected ed25519.PublicKey, server bool) *tls.Config {
	result := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate}, NextProtos: []string{profile}, SessionTicketsDisabled: true,
		InsecureSkipVerify: true}
	if server {
		result.ClientAuth = tls.RequireAnyClientCert
	}
	result.VerifyConnection = func(state tls.ConnectionState) error {
		if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != profile || len(state.PeerCertificates) != 1 {
			return errors.New("synthetic native TLS contract is invalid")
		}
		key, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || !key.Equal(expected) {
			return errors.New("synthetic native TLS peer key is invalid")
		}
		return nil
	}
	return result
}

func binding(index int, initiator bool, notAfter time.Time) route.LegBinding {
	first, second := identifier(4), identifier(5)
	firstRole, secondRole := byte(1), byte(3)
	if !initiator {
		first, second, firstRole, secondRole = second, first, secondRole, firstRole
	}
	return route.LegBinding{NetworkID: identifier(1), Digest: identifier(2), AttachmentID: identifier(byte(index + 10)),
		Epoch: 1, SenderRole: firstRole, PeerRole: secondRole, SenderNodeID: first, PeerNodeID: second, NotAfter: notAfter}
}

func identifier(value byte) (result [32]byte) { result[0] = value; return result }

func writeAll(output io.Writer, value []byte) error {
	for len(value) != 0 {
		written, err := output.Write(value)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		value = value[written:]
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
