//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func launchNativeSource(t *testing.T, binary, plan string) *nativeForwardingProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-source-%d-%d.service", os.Getpid(), time.Now().UnixNano())
	p := &nativeForwardingProcess{unit: unit}
	p.command = exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "source", "serve", plan)
	p.command.Stderr = &p.diagnostic
	var err error
	p.stdout, err = p.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !p.joined {
			if err := exec.Command("systemctl", "stop", unit).Run(); err != nil {
				t.Error("original Source stop unavailable", err)
			}
			_ = p.command.Wait()
			_ = p.stdout.Close()
			p.joined = true
		}
		if t.Failed() {
			t.Log("Source original diagnostic", p.diagnostic.String())
		}
	})
	return p
}

func TestNativeSourceAuthenticatedDistributionAndJoinedRetirement(t *testing.T) {
	for _, mode := range []string{"cancel", "protection-loss", "cpu-pressure"} {
		t.Run(mode, func(t *testing.T) { nativeSourceWorkload(t, mode) })
	}
}

func TestNativeSourceOriginalInputsRetire(t *testing.T) {
	for _, name := range []string{"plan", "certificate", "key", "ca"} {
		for _, fault := range []string{"replace", "mutate", "unlink", "access", "ancestor"} {
			t.Run(name+"/"+fault, func(t *testing.T) { nativeSourceWorkload(t, "input:"+name+":"+fault) })
		}
	}
}

func nativeSourceWorkload(t *testing.T, mode string) {
	binary := nodeNativeCommand(t)
	f := newNetworkAdmissionFixture(t)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := joinProcessClock(t)
	defer closeClock()
	networkPlan := seedNativeNodeState(t, f, bundle, clock)
	server, client, unauthorized, root, serverPin, clientPin := sourceNativeTLS(t)
	cert, key := nativeDescriptorCredential(t, server)
	rootPath := filepath.Join(t.TempDir(), "root.pem")
	if err := os.WriteFile(rootPath, root, 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	plan := sourceServePlan{Network: &networkPlan, ProcessProfile: processresource.SourceProfile,
		ParticipationRoot: filepath.Join(t.TempDir(), "participation"), Address: address, Certificate: cert, PrivateKey: key, ClientRoot: rootPath, ClientPins: [][32]byte{clientPin}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "source-plan")
	if err := os.WriteFile(planPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	inputPins := make(map[string][32]byte)
	for _, path := range []string{planPath, cert, key, rootPath, clock} {
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputPins[path] = sha256.Sum256(bytes)
		clear(bytes)
	}
	t.Logf("original Source configuration sha256=%x inputs=%d", inputPins[planPath], len(inputPins))
	p := launchNativeSource(t, binary, planPath)
	var ready map[string]string
	if err := json.NewDecoder(p.stdout).Decode(&ready); err != nil || ready["operation"] != "source.serve" || ready["phase"] != "listening" {
		t.Fatal("original Source startup", ready, err)
	}
	pid, err := exec.Command("systemctl", "show", p.unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(pid), "%d", &p.pid); err != nil || p.pid < 2 {
		t.Fatal("original Source MainPID unavailable", err)
	}
	public := f.spec.Authority.Public().(ed25519.PublicKey)
	authorities := map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}
	clientConfig := source.Config{Sources: [2]source.Source{
		{Address: address, ServerName: "source-native.test", Identity: [32]byte{71}, Family: "source-one", EndpointHandle: "source-one", RootPEM: root, LeafKeyDigest: serverPin},
		{Address: "127.0.0.1:9", ServerName: "unused.test", Identity: [32]byte{72}, Family: "source-two", EndpointHandle: "source-two", RootPEM: root, LeafKeyDigest: [32]byte{73}},
	}, ClientCertificate: client, VerificationClock: time.Now}
	fetch := func(config source.Config, message source.Message) (source.Message, error) {
		t.Helper()
		client, _, err := source.New(config, authorities)
		if err != nil {
			return source.Message{}, err
		}
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		return client.Fetch(ctx, 0, message)
	}
	for _, operation := range []string{"latest", "by-digest"} {
		request := source.Message{Operation: operation, NetworkDigest: source.NetworkDigest(networkPlan.NetworkID)}
		if operation == "by-digest" {
			request.ObjectDigest = bundle.Epoch.Digest
		}
		response, err := fetch(clientConfig, request)
		if err != nil || response.Status != "ok" || response.ObjectDigest != bundle.Epoch.Digest {
			t.Fatal("genuine authenticated exchange", operation, response.Status, err)
		}
		decoded, err := source.DecodeBundle(response.Payload)
		if err != nil || !bytes.Equal(decoded.Epoch, bundle.Epoch.Raw) || len(decoded.Materials) != 1 || !bytes.Equal(decoded.Materials[0], bundle.Epoch.Materials[0]) {
			t.Fatal("canonical signed byte oracle differs", err)
		}
		verified, err := epoch.Verify(epoch.Policy{NetworkID: networkPlan.NetworkID, Authorities: authorities, Threshold: 1, Profile: "ardents-route-v3", Now: time.Now()}, decoded.Epoch, decoded.Inputs, decoded.Materials, true)
		if err != nil || verified.Header.Digest != bundle.Epoch.Digest {
			t.Fatal("independent new Epoch verification", err)
		}
	}
	for _, fault := range []string{"network", "digest", "index", "pin", "hostname", "ca", "client", "client-ca"} {
		t.Run(fault, func(t *testing.T) {
			config := clientConfig
			request := source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(networkPlan.NetworkID)}
			switch fault {
			case "network":
				request.NetworkDigest[0] ^= 1
			case "digest":
				request.Operation = "by-digest"
				request.ObjectDigest = [32]byte{99}
			case "index":
				request.MaterialIndex = 63
			case "pin":
				config.Sources[0].LeafKeyDigest[0] ^= 1
			case "hostname":
				config.Sources[0].ServerName = "wrong.test"
			case "ca":
				_, _, _, config.Sources[0].RootPEM, _, _ = sourceNativeTLS(t)
			case "client":
				config.ClientCertificate = unauthorized
			case "client-ca":
				_, config.ClientCertificate, _, _, _, _ = sourceNativeTLS(t)
			}
			response, err := fetch(config, request)
			if err == nil && response.Status == "ok" {
				t.Fatal("causal refusal missing", fault)
			}
			if (fault == "pin" || fault == "hostname" || fault == "ca") && !errors.Is(err, source.ErrAuthentication) {
				t.Fatal("wrong causal authentication class", err)
			}
		})
	}
	for _, fault := range []string{"malformed", "truncated"} {
		t.Run(fault, func(t *testing.T) {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(root) {
				t.Fatal("fixture CA absent")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
			defer cancel()
			dialer := tls.Dialer{Config: &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, RootCAs: pool, ServerName: "source-native.test", Certificates: []tls.Certificate{client}}}
			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(4 * time.Second)); err != nil {
				t.Fatal(err)
			}
			request := make([]byte, 77)
			copy(request, "BROKEN!!")
			if fault == "truncated" {
				request = request[:8]
			}
			if _, err := conn.Write(request); err != nil {
				t.Fatal(err)
			}
			if err := conn.(*tls.Conn).CloseWrite(); err != nil {
				t.Fatal(err)
			}
			var response [45]byte
			if _, err := io.ReadFull(conn, response[:]); err != nil {
				t.Fatal("actual framed refusal unavailable", err)
			}
			if string(response[:8]) != "ARDH3S1\x00" || response[8] != 3 || !bytes.Equal(response[9:], make([]byte, 36)) {
				t.Fatal("malformed request gained signed bytes", response)
			}
		})
	}
	// Healthy exchange after all refusals proves the controls did not merely
	// observe a dead listener or retire the original for peer-local errors.
	if response, err := fetch(clientConfig, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(networkPlan.NetworkID)}); err != nil || response.Status != "ok" {
		t.Fatal("original Source lost after peer refusal", response.Status, err)
	}
	switch mode {
	case "cancel":
		if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGTERM", p.unit).Run(); err != nil {
			t.Fatal(err)
		}
	case "protection-loss":
		if err := exec.Command("systemctl", "set-property", "--runtime", p.unit, "CPUQuota=50%").Run(); err != nil {
			t.Fatal(err)
		}
	case "cpu-pressure":
		// Establish contention during actual bounded Source work, not only an
		// idle listener. The original limits and six-second pressure oracle stay.
		loadClient, _, err := source.New(clientConfig, authorities)
		if err != nil {
			t.Fatal(err)
		}
		loadCtx, stopLoad := context.WithTimeout(t.Context(), 8*time.Second)
		loadJoined := make(chan error, 1)
		go func() {
			for loadCtx.Err() == nil {
				response, err := loadClient.Fetch(loadCtx, 0, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(networkPlan.NetworkID)})
				if err != nil {
					loadJoined <- err
					return
				}
				if response.Status != "ok" {
					loadJoined <- errors.New("Source load retired")
					return
				}
			}
			loadJoined <- loadCtx.Err()
		}()
		t.Cleanup(func() { stopLoad(); <-loadJoined })
		nativeDescriptorCPUContention(t, p)
	}
	var restore func()
	if strings.HasPrefix(mode, "input:") {
		parts := strings.Split(mode, ":")
		path := map[string]string{"plan": planPath, "certificate": cert, "key": key, "ca": rootPath}[parts[1]]
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// This genuine original socket is held in the server's TLS read while
		// input loss occurs. Retirement must interrupt and join that handler.
		held, err := net.Dial("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Close()
		if _, err := held.Write([]byte{0x16}); err != nil {
			t.Fatal(err)
		}
		switch parts[2] {
		case "replace":
			err = os.Rename(path, path+".original")
			if err == nil {
				err = os.WriteFile(path, original, 0600)
			}
			restore = func() {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path+".original", path); err != nil {
					t.Fatal(err)
				}
			}
		case "mutate":
			changed := bytes.Clone(original)
			changed[len(changed)-1] ^= 1
			err = os.WriteFile(path, changed, 0600)
			clear(changed)
		case "unlink":
			err = os.Remove(path)
		case "access":
			err = os.Chmod(path, 0400)
		case "ancestor":
			parent := filepath.Dir(path)
			err = os.Rename(parent, parent+".original")
			restore = func() {
				if err := os.Rename(parent+".original", parent); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		if restore == nil {
			restore = func() {
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	err = p.command.Wait()
	p.joined = true
	_ = p.stdout.Close()
	var exited *exec.ExitError
	var terminal struct {
		Phase, Outcome string
		First          string `json:"first_owner"`
		Resource       *processresource.FailureFacts
	}
	expectedCode, expectedOutcome := 130, "canceled"
	if mode != "cancel" {
		expectedCode, expectedOutcome = 1, "failed"
	}
	if !errors.As(err, &exited) || exited.ExitCode() != expectedCode || json.Unmarshal(bytes.TrimSpace([]byte(p.diagnostic.String())), &terminal) != nil || terminal.Phase != "joined" || terminal.Outcome != expectedOutcome {
		t.Fatal("original Source joined outcome", err, p.diagnostic.String())
	}
	if mode != "cancel" && !strings.HasPrefix(mode, "input:") && terminal.First != "process" {
		t.Fatal("retained original process cause absent", p.diagnostic.String())
	}
	if strings.HasPrefix(mode, "input:") {
		if terminal.First != "input" {
			t.Fatal("original input refusal cause absent", p.diagnostic.String())
		}
		restore()
		if response, err := fetch(clientConfig, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(networkPlan.NetworkID)}); err == nil && response.Status == "ok" {
			t.Fatal("restored input revived retired Source")
		}
	}
	if mode == "cpu-pressure" && (terminal.Resource == nil || terminal.Resource.Cause != "cpu-pressure" || terminal.Resource.Observation == nil || terminal.Resource.Observation.CPUPressure < 1 || terminal.Resource.Observation.CPUWindowStarted.IsZero()) {
		t.Fatal("actual established overload absent", p.diagnostic.String())
	}
	config, err := networkStateConfig(&networkPlan)
	if err != nil {
		t.Fatal(err)
	}
	// Exclusive genuine reopen checks actual root release, not PID/path absence.
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal("original State lease not released", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	for path, expected := range inputPins {
		bytes, err := os.ReadFile(path)
		if err != nil || sha256.Sum256(bytes) != expected {
			t.Fatal("original Source configuration bytes changed", err)
		}
		clear(bytes)
	}
}
