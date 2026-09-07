//go:build ignore

// Disposable R-152 byte-accounting probe; not an Ardents Route implementation.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

type countingConn struct {
	net.Conn
	written atomic.Int64
}

func (c *countingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.written.Add(int64(n))
	return n, err
}

type exchangeCount struct {
	Index int   `json:"index"`
	Tx    int64 `json:"tx"`
	Rx    int64 `json:"rx"`
}

type stackResult struct {
	Layers      int             `json:"layers"`
	Sample      int             `json:"sample"`
	HandshakeTx int64           `json:"handshake_tx"`
	HandshakeRx int64           `json:"handshake_rx"`
	Exchanges   []exchangeCount `json:"exchanges"`
}

func certificate() (tls.Certificate, ed25519.PublicKey, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	at := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    at.Add(-time.Hour), NotAfter: at.Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}, public, nil
}

func measure(layers, sample int, request, response []byte) (stackResult, error) {
	result := stackResult{Layers: layers, Sample: sample}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	deadline := time.Now().Add(30 * time.Second)
	if err := left.SetDeadline(deadline); err != nil {
		return result, err
	}
	if err := right.SetDeadline(deadline); err != nil {
		return result, err
	}
	outerClient := &countingConn{Conn: left}
	outerServer := &countingConn{Conn: right}
	var client net.Conn = outerClient
	var server net.Conn = outerServer
	for level := 0; level < layers; level++ {
		cert, public, err := certificate()
		if err != nil {
			return result, err
		}
		clientTLS := tls.Client(client, &tls.Config{
			MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			SessionTicketsDisabled: true,
			InsecureSkipVerify:     true, // Exact generated key is verified below.
			VerifyConnection: func(state tls.ConnectionState) error {
				if state.Version != tls.VersionTLS13 || len(state.PeerCertificates) != 1 {
					return errors.New("invalid generated peer")
				}
				key, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
				if !ok || !bytes.Equal(key, public) {
					return errors.New("generated peer key mismatch")
				}
				return nil
			},
		})
		serverTLS := tls.Server(server, &tls.Config{
			MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			SessionTicketsDisabled: true, Certificates: []tls.Certificate{cert},
		})
		done := make(chan error, 2)
		go func() { done <- clientTLS.Handshake() }()
		go func() { done <- serverTLS.Handshake() }()
		first, second := <-done, <-done
		if err := errors.Join(first, second); err != nil {
			return result, err
		}
		client, server = clientTLS, serverTLS
	}
	result.HandshakeTx = outerClient.written.Load()
	result.HandshakeRx = outerServer.written.Load()
	for index := 0; index < 5; index++ {
		txBefore, rxBefore := outerClient.written.Load(), outerServer.written.Load()
		done := make(chan error, 1)
		go func() {
			got := make([]byte, len(request))
			if _, err := io.ReadFull(server, got); err != nil {
				done <- err
				return
			}
			if !bytes.Equal(got, request) {
				done <- errors.New("request mismatch")
				return
			}
			n, err := server.Write(response)
			if err == nil && n != len(response) {
				err = io.ErrShortWrite
			}
			done <- err
		}()
		n, sendErr := client.Write(request)
		if sendErr == nil && n != len(request) {
			sendErr = io.ErrShortWrite
		}
		got := make([]byte, len(response))
		_, readErr := io.ReadFull(client, got)
		serveErr := <-done
		if err := errors.Join(sendErr, readErr, serveErr); err != nil {
			return result, err
		}
		if !bytes.Equal(got, response) {
			return result, errors.New("response mismatch")
		}
		result.Exchanges = append(result.Exchanges, exchangeCount{
			Index: index + 1, Tx: outerClient.written.Load() - txBefore,
			Rx: outerServer.written.Load() - rxBefore,
		})
	}
	return result, nil
}

func run() error {
	request, response := make([]byte, 512), make([]byte, 65536)
	if _, err := rand.Read(request); err != nil {
		return err
	}
	if _, err := rand.Read(response); err != nil {
		return err
	}
	report := struct {
		Kind          string        `json:"kind"`
		Go            string        `json:"go"`
		Platform      string        `json:"platform"`
		Started       string        `json:"started"`
		RequestBytes  int           `json:"request_bytes"`
		ResponseBytes int           `json:"response_bytes"`
		Results       []stackResult `json:"results"`
	}{
		Kind: "nested-tls-record-bytes-only",
		Go:   runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH,
		Started:      time.Now().UTC().Format(time.RFC3339),
		RequestBytes: len(request), ResponseBytes: len(response),
	}
	for layers := 1; layers <= 4; layers++ {
		for sample := 1; sample <= 5; sample++ {
			result, err := measure(layers, sample, request, response)
			if err != nil {
				return fmt.Errorf("layers=%d sample=%d: %w", layers, sample, err)
			}
			report.Results = append(report.Results, result)
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
