//go:build ignore

package main

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"runtime"
	"time"
)

type clientArguments struct {
	endpoint string
	side     string
	token    string
	deadline time.Time
	hold     time.Duration
}

type clientResult struct {
	Schema          string `json:"schema"`
	Role            string `json:"role"`
	Side            string `json:"side"`
	Outcome         string `json:"outcome"`
	PayloadBytes    int    `json:"payload_bytes"`
	PayloadSHA256   string `json:"payload_sha256"`
	GoroutinesStart int    `json:"goroutines_start"`
	GoroutinesEnd   int    `json:"goroutines_end"`
	FDsStart        int    `json:"fds_start"`
	FDsEnd          int    `json:"fds_end"`
	CleanupJoined   bool   `json:"cleanup_joined"`
	ElapsedMS       int64  `json:"elapsed_ms"`
}

func runClient(ctx context.Context, arguments clientArguments) (clientResult, error) {
	started := time.Now()
	startGoroutines, startFDs := runtime.NumGoroutine(), fileDescriptorCount()
	material, err := deterministicIdentities()
	if err != nil {
		return clientResult{}, err
	}
	config, senderID, err := clientTLS(material, arguments.side)
	if err != nil {
		return clientResult{}, err
	}
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", arguments.endpoint)
	if err != nil {
		return clientResult{}, err
	}
	connection := tls.Client(raw, config)
	defer connection.Close()
	if err := connection.SetDeadline(arguments.deadline); err != nil {
		return clientResult{}, err
	}
	if err := connection.HandshakeContext(ctx); err != nil {
		return clientResult{}, err
	}
	state := connection.ConnectionState()
	if state.NegotiatedProtocol != nativeALPN || len(state.PeerCertificates) == 0 ||
		publicIdentity(state.PeerCertificates[0]) != material.serverID {
		return clientResult{}, errors.New("server TLS identity or ALPN is unauthorized")
	}
	binding, err := clientBinding(arguments.side, arguments.token, senderID, material.serverID, arguments.deadline)
	if err != nil {
		return clientResult{}, err
	}
	if err := writeBinding(connection, binding); err != nil {
		return clientResult{}, err
	}
	peer, err := readBinding(connection)
	if err != nil {
		return clientResult{}, err
	}
	if err := binding.VerifyReciprocal(peer); err != nil {
		return clientResult{}, err
	}
	if arguments.hold > 0 {
		timer := time.NewTimer(arguments.hold)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return clientResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	if arguments.side == "initiator" {
		if err := writeTranscript(connection); err != nil {
			return clientResult{}, err
		}
		if err := readTranscript(connection); err != nil {
			return clientResult{}, err
		}
	} else {
		if err := readTranscript(connection); err != nil {
			return clientResult{}, err
		}
		if err := writeTranscript(connection); err != nil {
			return clientResult{}, err
		}
	}
	if err := connection.Close(); err != nil {
		return clientResult{}, err
	}
	endGoroutines, endFDs, joined := waitForClientCleanup(startGoroutines, startFDs)
	return clientResult{Schema: "ardents-r092-rendezvous-client-v1", Role: "client", Side: arguments.side,
		Outcome: "success", PayloadBytes: transcriptBytes,
		PayloadSHA256: hex.EncodeToString(expectedTranscriptDigest[:]), GoroutinesStart: startGoroutines,
		GoroutinesEnd: endGoroutines, FDsStart: startFDs, FDsEnd: endFDs, CleanupJoined: joined,
		ElapsedMS: time.Since(started).Milliseconds()}, nil
}

func fileDescriptorCount() int {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return -1
	}
	return len(entries)
}

func waitForClientCleanup(startGoroutines, startFDs int) (int, int, bool) {
	deadline := time.Now().Add(time.Second)
	for {
		goroutines, descriptors := runtime.NumGoroutine(), fileDescriptorCount()
		if goroutines <= startGoroutines+1 && (startFDs < 0 || descriptors <= startFDs) {
			return goroutines, descriptors, true
		}
		if time.Now().After(deadline) {
			return goroutines, descriptors, false
		}
		time.Sleep(10 * time.Millisecond)
	}
}
