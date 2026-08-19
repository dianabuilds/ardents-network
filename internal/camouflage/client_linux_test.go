//go:build linux

package camouflage_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/camouflage"
)

func TestPinnedWebTunnelCarriesUsefulWorkAndCleansRepeatedly(t *testing.T) {
	clientPath := os.Getenv("ARDENTS_WEBTUNNEL_CLIENT")
	serverPath := os.Getenv("ARDENTS_WEBTUNNEL_SERVER")
	if clientPath == "" || serverPath == "" {
		t.Skip("ARDENTS_WEBTUNNEL_CLIENT and ARDENTS_WEBTUNNEL_SERVER name prepared pinned binaries")
	}
	root := t.TempDir()
	certificatePath, keyPath, pin := writeTLSIdentity(t, root)
	config, err := camouflage.Validate(candidateEnvelopeWithPin(8443, pin), [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	echo, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	echoDone := make(chan error, 1)
	go func() {
		connection, err := echo.Accept()
		if err != nil {
			echoDone <- err
			return
		}
		defer connection.Close()
		request := make([]byte, 4)
		if _, err = io.ReadFull(connection, request); err == nil && string(request) != "ping" {
			err = errors.New("next leg received changed bytes")
		}
		if err == nil {
			_, err = connection.Write([]byte("pong"))
		}
		echoDone <- err
	}()
	serverState := filepath.Join(root, "server-state")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serving, err := camouflage.Serve(ctx, config, camouflage.Server{
		Binary: serverPath, StateRoot: serverState, Certificate: certificatePath, Key: keyPath,
		NextLeg: echo.Addr().String(), Deadline: time.Now().Add(5 * time.Second), ResourceProfile: "h3-s-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	for episode := range 20 {
		t.Run("uninformed-probe-"+strconv.Itoa(episode), func(t *testing.T) {
			assertUninformedProbes(t, certificatePath)
		})
	}
	clientState := filepath.Join(root, "client-state")
	carrier, cleanup, _, err := camouflage.OpenClient(ctx, config, camouflage.Client{
		Binary: clientPath, StateRoot: clientState, Deadline: time.Now().Add(5 * time.Second),
	})
	if err != nil {
		_ = serving.Close()
		t.Fatal(err)
	}
	serving.Protect(true)
	blocked, dialErr := net.DialTimeout("tcp4", "203.0.113.7:8443", time.Second)
	if dialErr == nil {
		_ = blocked.SetDeadline(time.Now().Add(250 * time.Millisecond))
		_, _ = blocked.Write([]byte("new admission"))
		one := make([]byte, 1)
		if _, readErr := blocked.Read(one); readErr == nil {
			t.Fatal("protected front admitted new work")
		}
		_ = blocked.Close()
	}
	if _, err := carrier.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(carrier, response); err != nil || string(response) != "pong" {
		t.Fatalf("carrier response = %q, error = %v", response, err)
	}
	if err := <-echoDone; err != nil {
		t.Fatal(err)
	}
	serving.Protect(false)
	clientPIDs := candidatePIDs(t, clientPath)
	serverPIDs := candidatePIDs(t, serverPath)
	assertCandidateBounds(t, clientPIDs, 16, 4)
	assertCandidateBounds(t, serverPIDs, 64, 32)
	assertStateBound(t, clientState)
	assertStateBound(t, serverState)
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("repeated client cleanup: %v", err)
	}
	if err := serving.Close(); err != nil {
		t.Fatal(err)
	}
	if err := serving.Close(); err != nil {
		t.Fatalf("repeated server cleanup: %v", err)
	}
	for _, item := range []struct {
		binary string
		state  string
	}{{clientPath, clientState}, {serverPath, serverState}} {
		if remaining := candidatePIDs(t, item.binary); len(remaining) != 0 {
			t.Fatalf("candidate process residue = %v", remaining)
		}
		if _, err := os.Stat(item.state); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("candidate state residue: %v", err)
		}
	}
	assertPIDsGone(t, append(clientPIDs, serverPIDs...))
	rebound, err := net.Listen("tcp4", "203.0.113.7:8443")
	if err != nil {
		t.Fatalf("adapter front listener residue: %v", err)
	}
	_ = rebound.Close()
}

func assertUninformedProbes(t *testing.T, certificatePath string) {
	t.Helper()
	certificate, err := os.ReadFile(certificatePath)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificate) {
		t.Fatal("front certificate was not accepted as a probe root")
	}
	tlsConfig := &tls.Config{RootCAs: roots, ServerName: "front.example", MinVersion: tls.VersionTLS13}
	client := &http.Client{Transport: &http.Transport{Proxy: nil, TLSClientConfig: tlsConfig}, Timeout: 5 * time.Second}
	for _, path := range []string{"/", "/missing"} {
		request, err := http.NewRequest(http.MethodGet, "https://203.0.113.7:8443"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Host = "front.example"
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1024))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusNotFound {
			t.Fatalf("uninformed path %q = %d %q, %v %v", path, response.StatusCode, body, readErr, closeErr)
		}
		lower := strings.ToLower(string(body))
		if strings.Contains(lower, "ardents") || strings.Contains(lower, "webtunnel") || strings.Contains(lower, "cmethod") {
			t.Fatalf("uninformed response exposed an implementation identifier: %q", body)
		}
	}
	raw, err := net.DialTimeout("tcp4", "203.0.113.7:8443", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = raw.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = raw.Write([]byte("not tls"))
	one := make([]byte, 1)
	if _, err := raw.Read(one); isTimeout(err) {
		t.Fatal("malformed TLS was not closed or alerted within 5 seconds")
	}
	_ = raw.Close()
	dialer := &net.Dialer{Timeout: time.Second}
	secured, err := tls.DialWithDialer(dialer, "tcp4", "203.0.113.7:8443", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	_ = secured.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = secured.Write([]byte("BROKEN\x00HTTP\r\n\r\n"))
	response := make([]byte, 64)
	count, readErr := secured.Read(response)
	_ = secured.Close()
	if isTimeout(readErr) || readErr == nil && !bytes.Contains(response[:count], []byte(" 400 ")) {
		t.Fatalf("malformed HTTP returned an unexpected oracle: %q", response[:count])
	}
}

func isTimeout(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func TestPinnedClientUsesSanitizedPTAndOneNumericDialBeforeRefusal(t *testing.T) {
	binaryPath := os.Getenv("ARDENTS_WEBTUNNEL_CLIENT")
	if binaryPath == "" {
		t.Skip("ARDENTS_WEBTUNNEL_CLIENT names the externally prepared pinned binary")
	}
	listener, err := net.Listen("tcp4", "203.0.113.7:8443")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	envelope := candidateEnvelopeAt(8443)
	config, err := camouflage.Validate(envelope, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(t.TempDir(), "candidate-state")
	type openResult struct {
		err error
	}
	result := make(chan openResult, 1)
	go func() {
		carrier, cleanup, _, err := camouflage.OpenClient(context.Background(), config, camouflage.Client{
			Binary: binaryPath, StateRoot: stateRoot, Deadline: time.Now().Add(5 * time.Second),
		})
		if carrier != nil {
			_ = carrier.Close()
		}
		if cleanup != nil {
			err = errors.Join(err, cleanup())
		}
		result <- openResult{err: err}
	}()
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	pids := candidatePIDs(t, binaryPath)
	if len(pids) != 1 {
		t.Fatalf("candidate process count = %d, want 1", len(pids))
	}
	assertCandidateBounds(t, pids, 16, 4)
	wantEnvironment := []string{
		"TOR_PT_CLIENT_TRANSPORTS=webtunnel",
		"TOR_PT_EXIT_ON_STDIN_CLOSE=1",
		"TOR_PT_MANAGED_TRANSPORT_VER=1",
		"TOR_PT_STATE_LOCATION=" + stateRoot,
	}
	observedEnvironment := processEnvironment(t, pids[0])
	if !equalStrings(observedEnvironment, wantEnvironment) {
		t.Fatalf("candidate environment = %q, want %q", observedEnvironment, wantEnvironment)
	}
	_ = connection.Close()
	opened := <-result
	if opened.err == nil || !strings.HasPrefix(opened.err.Error(), "adapter-socks-refused:") {
		t.Fatalf("OpenClient() error = %v, want adapter-socks-refused", opened.err)
	}
	if remaining := candidatePIDs(t, binaryPath); len(remaining) != 0 {
		t.Fatalf("candidate process residue = %v", remaining)
	}
	if _, err := os.Stat(stateRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("candidate state residue: %v", err)
	}
	if variant := os.Getenv("ARDENTS_G7_VARIANT"); variant != "" {
		components := map[string]string{"dns": "adapter-resolver", "environment-proxy": "adapter-process",
			"alternate-address": "adapter-config", "alternate-candidate": "bridge-ledger",
			"cached-success": "adapter-state", "deadline-exposure-reset": "bridge-attempt"}
		component, ok := components[variant]
		if !ok {
			t.Fatalf("unsupported G7 adapter contract variant %q", variant)
		}
		rawInput, _ := json.Marshal(struct {
			CandidateEnvelope string   `json:"candidate_envelope"`
			AmbientProxy      []string `json:"ambient_proxy"`
		}{hex.EncodeToString(envelope), []string{os.Getenv("HTTP_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("ALL_PROXY")}})
		contract, _ := json.Marshal(struct {
			Schema           string          `json:"schema"`
			Variant          string          `json:"variant"`
			Component        string          `json:"component"`
			Input            json.RawMessage `json:"input"`
			ReachableTargets []string        `json:"reachable_targets"`
			ObservedTargets  []string        `json:"observed_targets"`
			ChildEnvironment []string        `json:"child_environment"`
			StateEntries     []string        `json:"state_entries"`
			EntryError       string          `json:"entry_error"`
		}{"ardents-h3-g7-component-v1", variant, component, rawInput,
			[]string{"203.0.113.7:8443"}, []string{"203.0.113.7:8443"}, observedEnvironment, []string{}, ""})
		fmt.Printf("g7-component-contract=%s\n", contract)
	}
}

func assertCandidateBounds(t *testing.T, pids []int, maximumFDs, maximumSockets int) {
	t.Helper()
	if len(pids) != 1 {
		t.Fatalf("candidate process count = %d, want 1", len(pids))
	}
	status, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pids[0]), "status"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(status), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "Uid:" && fields[2] == "0" {
			t.Fatal("candidate process retained effective UID 0")
		}
	}
	entries, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pids[0]), "fd"))
	if err != nil {
		t.Fatal(err)
	}
	sockets := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pids[0]), "fd", entry.Name()))
		if err == nil && strings.HasPrefix(target, "socket:[") {
			sockets++
		}
	}
	if len(entries) > maximumFDs || sockets > maximumSockets {
		t.Fatalf("candidate resources = %d FDs/%d sockets", len(entries), sockets)
	}
}

func assertStateBound(t *testing.T, root string) {
	t.Helper()
	entries, bytes := 0, int64(0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		entries++
		if !entry.IsDir() {
			info, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			bytes += info.Size()
		}
		return nil
	})
	if err != nil || entries > 32 || bytes > 1<<20 {
		t.Fatalf("candidate state = %d entries/%d bytes, error = %v", entries, bytes, err)
	}
}

func assertPIDsGone(t *testing.T, pids []int) {
	t.Helper()
	for _, pid := range pids {
		if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("candidate PID %d residue: %v", pid, err)
		}
	}
}

func candidateEnvelopeAt(port uint16) []byte {
	raw := candidateEnvelope()
	offset := len("ardents-h3-wt1") + 1 + 1 + len("webtunnel-v0.0.6") + 4
	binary.BigEndian.PutUint16(raw[offset:offset+2], port)
	return raw
}

func candidateEnvelopeWithPin(port uint16, pin [32]byte) []byte {
	raw := candidateEnvelopeAt(port)
	copy(raw[len(raw)-32:], pin[:])
	return raw
}

func writeTLSIdentity(t *testing.T, root string) (string, string, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "front.example"},
		DNSNames: []string{"front.example"}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	certificatePath := filepath.Join(root, "front.pem")
	keyPath := filepath.Join(root, "front-key.pem")
	if err := os.WriteFile(certificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certificatePath, keyPath, sha256.Sum256(der)
}

func candidatePIDs(t *testing.T, binaryPath string) []int {
	t.Helper()
	want, err := filepath.EvalSymlinks(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatal(err)
	}
	var found []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		executable, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err == nil && executable == want {
			found = append(found, pid)
		}
	}
	return found
}

func processEnvironment(t *testing.T, pid int) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "environ"))
	if err != nil {
		t.Fatal(err)
	}
	values := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	sort.Strings(values)
	return values
}

func equalStrings(left, right []string) bool {
	return bytes.Equal([]byte(strings.Join(left, "\x00")), []byte(strings.Join(right, "\x00")))
}
