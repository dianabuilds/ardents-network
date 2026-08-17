//go:build linux && live

package network_test

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type blockedProbePlan struct {
	Address    string `json:"address"`
	ServerName string `json:"server_name"`
	Path       string `json:"path"`
	Envelope   string `json:"envelope"`
	Identity   string `json:"identity"`
}

func proveBlockedPolicy(t *testing.T) blockedTimeline {
	t.Helper()
	profile := os.Getenv("ARDENTS_BLOCKED_PROFILE")
	if profile != "C1" && profile != "C2" {
		t.Fatalf("unknown blocked profile %q", profile)
	}
	var timeline blockedTimeline
	readBlockedJSON(t, filepath.Join(blockedSync(), "timeline-start.json"), &timeline)
	if timeline.ManifestStartNS == 0 {
		t.Fatal("blocked manifest timeline is absent")
	}
	started := time.Now()
	ordinaryErr := blockedRejectedDial("172.31.20.11:4601", 3*time.Second)
	if ordinaryErr == nil {
		t.Fatal("ordinary entry unexpectedly accepted useful work")
	}
	if remaining := 3*time.Second - time.Since(started); remaining > 0 {
		time.Sleep(remaining)
	}
	rejected := []string{"172.31.20.11:4601"}
	if profile == "C2" {
		alternatives := []struct {
			name    string
			payload []byte
		}{
			{name: "raw", payload: []byte("ARDENTS-RAW-CARRIER\n")},
			{name: "pt", payload: []byte("CMETHOD webtunnel socks5 127.0.0.1:1\nCMETHODS DONE\n")},
		}
		for _, alternative := range alternatives {
			if err := blockedRejectedPayload("203.0.113.8:8480", alternative.payload, 2*time.Second); err != nil {
				t.Fatalf("undeclared %s alternative was not rejected by the declared boundary: %v",
					alternative.name, err)
			}
			rejected = append(rejected, "203.0.113.8:8480/"+alternative.name)
		}
	}
	timeline.ConditionNS = blockedMonotonicNS(t)
	result := map[string]any{"kind": "blocked-condition", "profile": profile,
		"ordinary_useful_bytes": 0, "observation_ms": time.Since(started).Milliseconds(), "rejected": rejected,
		"manifest_start_ns": timeline.ManifestStartNS, "condition_ns": timeline.ConditionNS}
	if profile == "C2" {
		result["boundary_allowlist"] = "tls13-http-webtunnel-v0.0.6"
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockedSync(), "blocked-condition.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(raw))
	return timeline
}

func blockedRejectedPayload(address string, payload []byte, bound time.Duration) error {
	connection, err := net.DialTimeout("tcp4", address, bound)
	if err != nil {
		return fmt.Errorf("declared boundary was unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(bound))
	if _, err := connection.Write(payload); err != nil {
		return nil
	}
	response, readErr := io.ReadAll(io.LimitReader(connection, 256))
	if isBlockedTimeout(readErr) {
		return errors.New("alternative remained open")
	}
	if bytesContainIdentifier(response) {
		return errors.New("alternative received an identifying response")
	}
	return nil
}

func blockedRejectedDial(address string, bound time.Duration) error {
	connection, err := net.DialTimeout("tcp4", address, bound)
	if err == nil {
		_ = connection.Close()
	}
	return err
}

func runBlockedProbe(t *testing.T) {
	t.Helper()
	prepareBlockedObservation(t, blockedManifest("probe"))
	var plan blockedProbePlan
	readBlockedJSON(t, "/run/secure/probe.json", &plan)
	certificate, err := os.ReadFile("/run/secure/front-cert.pem")
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certificate) {
		t.Fatal("probe front certificate is invalid")
	}
	seed, err := os.ReadFile("/run/secure/corpus-seed.bin")
	if err != nil || len(seed) != 32 {
		t.Fatalf("probe corpus seed is invalid: %v", err)
	}
	tlsConfig := &tls.Config{RootCAs: pool, ServerName: plan.ServerName, MinVersion: tls.VersionTLS13}
	profile := os.Getenv("ARDENTS_BLOCKED_PROBE_PROFILE")
	if profile == "C5" {
		for index, path := range []string{"/", "/wrong-path"} {
			status := blockedProbeHTTP(t, plan.Address, path, tlsConfig, probeCanary(seed, byte(index)))
			if !strings.Contains(status, " 404 ") {
				t.Fatalf("uninformed %s status = %q", path, status)
			}
		}
		blockedMalformedTLS(t, plan.Address, probeCanary(seed, 2))
		blockedMalformedHTTP(t, plan.Address, tlsConfig, probeCanary(seed, 3))
		fmt.Println(`{"kind":"probe-result","profile":"C5","requests":4}`)
	} else if profile == "C6" {
		informedStatus := blockedProbeHTTP(t, plan.Address, plan.Path, tlsConfig, probeCanary(seed, 4))
		if strings.Contains(informedStatus, " 404 ") {
			t.Fatalf("disclosed path was not detected: %q", informedStatus)
		}
		fmt.Printf("{\"kind\":\"probe-result\",\"profile\":\"C6\",\"disclosed_path\":\"detected\",\"status\":%q}\n",
			strings.TrimSpace(informedStatus))
	} else {
		t.Fatalf("unknown blocked probe profile %q", profile)
	}
	finishBlockedObservation(t)
}

func blockedProbeHTTP(t *testing.T, address, path string, config *tls.Config, canary [32]byte) string {
	t.Helper()
	connection, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp4", address, config.Clone())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintf(connection,
		"GET %s HTTP/1.1\r\nHost: front.example\r\nX-Ardents-Probe: %x\r\nConnection: close\r\n\r\n",
		path, canary); err != nil {
		t.Fatal(err)
	}
	status, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func blockedMalformedTLS(t *testing.T, address string, canary [32]byte) {
	t.Helper()
	started := time.Now()
	connection, err := net.DialTimeout("tcp4", address, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = connection.Write(append([]byte("not-tls"), canary[:]...))
	response, readErr := io.ReadAll(io.LimitReader(connection, 256))
	if isBlockedTimeout(readErr) || time.Since(started) > 5*time.Second {
		t.Fatalf("malformed TLS did not alert or close within 5s: %v", readErr)
	}
	if bytesContainIdentifier(response) {
		t.Fatal("malformed TLS exposed an Ardents identifier")
	}
}

func blockedMalformedHTTP(t *testing.T, address string, config *tls.Config, canary [32]byte) {
	t.Helper()
	started := time.Now()
	connection, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp4", address, config.Clone())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintf(connection, "BROKEN\x00HTTP\r\nX-Ardents-Probe: %x\r\n\r\n", canary); err != nil {
		t.Fatal(err)
	}
	response, readErr := io.ReadAll(io.LimitReader(connection, 512))
	if isBlockedTimeout(readErr) || time.Since(started) > 5*time.Second {
		t.Fatalf("malformed HTTP did not reject or close within 5s: %v", readErr)
	}
	if len(response) != 0 && !bytes.Contains(response, []byte(" 4")) {
		t.Fatalf("malformed HTTP response is neither 4xx nor close: %q", response)
	}
	if bytesContainIdentifier(response) {
		t.Fatal("malformed HTTP exposed an Ardents identifier")
	}
}

func probeCanary(seed []byte, ordinal byte) [32]byte {
	return sha256.Sum256(append(append([]byte(nil), seed...), ordinal))
}

func bytesContainIdentifier(value []byte) bool {
	lower := strings.ToLower(string(value))
	return strings.Contains(lower, "ardents") || strings.Contains(lower, "webtunnel") || strings.Contains(lower, "cmethod")
}

func isBlockedTimeout(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}
