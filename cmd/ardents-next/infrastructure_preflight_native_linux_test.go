//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// The native controller supplies an independently hashed canonical executable.
// No fixture command, test binary or synthesized success can substitute for it.
func sourcePreflightCommand(t *testing.T) []byte {
	t.Helper()
	path := os.Getenv("ARDENTS_SOURCE_PREFLIGHT_COMMAND")
	pin := os.Getenv("ARDENTS_SOURCE_PREFLIGHT_COMMAND_SHA256")
	raw, err := os.ReadFile(path)
	if err != nil || !filepath.IsAbs(path) || len(pin) != 64 || fmt.Sprintf("%x", sha256.Sum256(raw)) != pin {
		t.Fatal("invalid environment: independently pinned canonical command required", err)
	}
	t.Logf("actual Source preflight compiled consumer sha256=%x", sha256.Sum256(raw))
	return raw
}

func sourcePreflightFixture(t *testing.T, retain ...bool) (infrastructure.SourceRequest, string) {
	t.Helper()
	base := os.Getenv("ARDENTS_SOURCE_PREFLIGHT_FUTURE_PARENT")
	if !filepath.IsAbs(base) || os.Geteuid() != 0 {
		t.Fatal("invalid environment: isolated native future parent/root required")
	}
	parent, err := os.MkdirTemp(base, "source-preflight-")
	if err != nil {
		t.Fatal(err)
	}
	// Only this owned fixture parent is made traversable. Its existing outer
	// ancestors must already support the future non-root invocation.
	if err = os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if len(retain) == 0 {
		t.Cleanup(func() {
			if err := os.RemoveAll(parent); err != nil {
				t.Error(err)
			}
		})
	}
	inputs := filepath.Join(parent, "inputs")
	if err = os.Mkdir(inputs, 0700); err != nil {
		t.Fatal(err)
	}
	r := infrastructure.SourceRequest{Schema: "ardents-source-preflight-v1", Invocation: "source", Profile: "ardents-source-process-v1", InstallationRoot: filepath.Join(parent, "installation"), StateRoot: filepath.Join(parent, "state"), ParticipationRoot: filepath.Join(parent, "participation"), CredentialDirectory: filepath.Join(parent, "credentials"), Plan: filepath.Join(inputs, "plan"), Certificate: filepath.Join(inputs, "certificate"), PrivateKey: filepath.Join(inputs, "key"), ClientCA: filepath.Join(inputs, "ca"), Clock: filepath.Join(parent, "clock"), Address: "127.0.0.1:4567"}
	server, _, _, ca, _, clientPin := sourceNativeTLS(t)
	writeSigningTestKey(t, r.PrivateKey, server.PrivateKey.(ed25519.PrivateKey))
	for path, raw := range map[string][]byte{r.Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate[0]}), r.ClientCA: ca} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan := sourceServePlan{Network: &networkAuthorityPlan{Root: r.StateRoot, NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: r.Clock}, ProcessProfile: r.Profile, ParticipationRoot: r.ParticipationRoot, Address: r.Address, Certificate: filepath.Join(r.CredentialDirectory, "certificate.pem"), PrivateKey: filepath.Join(r.CredentialDirectory, "private-key.pem"), ClientRoot: filepath.Join(r.CredentialDirectory, "client-ca.pem"), ClientPins: [][32]byte{clientPin}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(r.Plan, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(r.Clock, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(r.Clock, 0644); err != nil {
		t.Fatal(err)
	}
	request := filepath.Join(inputs, "request")
	writeSourcePreflightRequest(t, request, r)
	return r, request
}

func writeSourcePreflightRequest(t *testing.T, path string, r infrastructure.SourceRequest) {
	t.Helper()
	// Literal field order and spelling are independent of the product decoder.
	raw := fmt.Sprintf("{\"schema\":%q,\"invocation\":%q,\"profile\":%q,\"installation_root\":%q,\"state_root\":%q,\"participation_root\":%q,\"credential_directory\":%q,\"plan\":%q,\"certificate\":%q,\"private_key\":%q,\"client_ca\":%q,\"clock\":%q,\"address\":%q}\n", r.Schema, r.Invocation, r.Profile, r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.CredentialDirectory, r.Plan, r.Certificate, r.PrivateKey, r.ClientCA, r.Clock, r.Address)
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSourcePreflightNativeCompiledInitialCandidateAndNoEffects(t *testing.T) {
	binary := sourcePreflightCommand(t)
	r, request := sourcePreflightFixture(t)
	dir, pin, _ := infrastructureDistribution(t, binary, nil, 1, nil, []byte(infrastructure.SourceTemplate))
	history := filepath.Join(filepath.Dir(dir), "history")
	program := filepath.Join(dir, "ardents-node-linux-amd64")
	args := []string{"infrastructure", "preflight-initial", dir, pin, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339), request}
	run := func(args []string, want bool) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), program, args...)
		out, err := cmd.CombinedOutput()
		if (err == nil) != want {
			if want {
				diagnoseSourcePreflight(t, dir, history, request)
			}
			t.Fatalf("actual preflight wanted success=%v: %s %v", want, out, err)
		}
		if want && !bytes.Equal(out, []byte("{\"outcome\":\"source-read-only-preflight-observed\"}\n")) {
			t.Fatal("public receipt is not bounded", string(out))
		}
		for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.CredentialDirectory} {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatal("preflight created runtime/installation destination", err)
			}
		}
	}
	beforePasswd, err := os.ReadFile("/etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	beforeGroup, err := os.ReadFile("/etc/group")
	if err != nil {
		t.Fatal(err)
	}
	run(args, true)
	run(args, false) // Retained floors cannot recreate first-pin provenance.
	candidate := []string{"infrastructure", "preflight-candidate", dir, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339), request}
	run(candidate, true)
	// The clock is a live root-writer declaration, not a pinned snapshot. An
	// independent fresh inode/content is permitted, but changed access refuses.
	clockReplacement := r.Clock + "-replacement"
	if err := os.WriteFile(clockReplacement, []byte("new evidence"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(clockReplacement, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(clockReplacement, r.Clock); err != nil {
		t.Fatal(err)
	}
	run(candidate, true)
	if err := os.Chmod(r.Clock, 0666); err != nil {
		t.Fatal(err)
	}
	run(candidate, false)
	if err := os.Chmod(r.Clock, 0644); err != nil {
		t.Fatal(err)
	}
	run(candidate, true)
	if err := os.Chmod(filepath.Dir(r.StateRoot), 0700); err != nil {
		t.Fatal(err)
	}
	run(candidate, false)
	if err := os.Chmod(filepath.Dir(r.StateRoot), 0755); err != nil {
		t.Fatal(err)
	}
	run(candidate, true)
	// Fill the actual original output pipe before launching the genuine compiled
	// consumer. Its bounded report must fail and physically return, not report
	// success merely because the earlier observations were healthy.
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	fd := int(writer.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	for {
		_, err := syscall.Write(fd, make([]byte, 4096))
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	blocked := exec.CommandContext(t.Context(), program, candidate...)
	blocked.Stdout = writer
	if err := blocked.Run(); err == nil {
		t.Fatal("blocked original receipt produced success")
	}
	run(candidate, true)
	for _, mutate := range []func(*infrastructure.SourceRequest){
		func(v *infrastructure.SourceRequest) { v.Invocation = "join" },
		func(v *infrastructure.SourceRequest) { v.Profile = "endpoint" },
		func(v *infrastructure.SourceRequest) { v.StateRoot = v.InstallationRoot + "/state" },
		func(v *infrastructure.SourceRequest) { v.Address = "127.0.0.1:443" },
		func(v *infrastructure.SourceRequest) { v.Address = "127.0.0.1:4568" },
	} {
		copy := r
		mutate(&copy)
		writeSourcePreflightRequest(t, request, copy)
		run(candidate, false)
	}
	writeSourcePreflightRequest(t, request, r)
	if err := os.Mkdir(r.StateRoot, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), program, candidate...)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("foreign existing runtime root adopted %s", out)
	}
	if err := os.Remove(r.StateRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(r.PrivateKey, 0644); err != nil {
		t.Fatal(err)
	}
	run(candidate, false)
	if err := os.Chmod(r.PrivateKey, 0600); err != nil {
		t.Fatal(err)
	}
	run(candidate, true)
	// The source and future root cannot alias Release floor effects, even before
	// a floor exists. No authentication may create the requested State root.
	bad := append([]string(nil), args...)
	bad[4] = r.StateRoot
	run(bad, false)
	for _, path := range []string{"/etc/passwd", "/etc/group"} {
		now, err := os.ReadFile(path)
		before := beforePasswd
		if strings.HasSuffix(path, "group") {
			before = beforeGroup
		}
		if err != nil || !bytes.Equal(before, now) {
			t.Fatal("native account namespace changed", err)
		}
	}
	for _, path := range []string{"/etc/systemd/system/ardents-source.service", "/run/systemd/system/ardents-source.service"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("preflight published unit", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if code := runInfrastructure(ctx, args[1:], new(bytes.Buffer)); code == 0 {
		t.Fatal("canceled original caller observed success")
	}
}

func diagnoseSourcePreflight(t *testing.T, dir, history, request string) {
	t.Helper()
	inputs, err := infrastructure.ReadSourceInputs(t.Context(), request)
	if err != nil {
		t.Log("original inputs", err, "joined input cleanup", inputs.Close())
		return
	}
	defer func() {
		if err := inputs.Close(); err != nil {
			t.Log("joined original input cleanup", err)
		}
	}()
	c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
	if err != nil {
		t.Log("candidate", err)
		return
	}
	v, err := release.OpenRetained(history)
	if err != nil {
		t.Log("retained Release", err)
		return
	}
	defer v.Close()
	a, err := infrastructure.AuthenticateCandidate(t.Context(), v, c, infrastructureCandidateInput(t, c))
	if err != nil {
		t.Log("candidate authentication", err)
		return
	}
	t.Log("local original native diagnosis", infrastructure.ObserveSource(v, a, inputs, dir, history))
}

func TestSourcePreflightNativeIsolatedForeignAccountUnitAndAlias(t *testing.T) {
	// These fixtures may change only the explicitly prepared private rootfs.
	// A missing marker is an invalid selected environment, never a passing skip.
	marker, err := os.ReadFile("/etc/ardents-source-preflight-native-stand")
	container, ce := os.ReadFile("/run/systemd/container")
	if err != nil || ce != nil || string(marker) != "ardents-source-preflight-native-v1\n" || strings.TrimSpace(string(container)) != "ardents-preflight" {
		t.Fatal("invalid environment: original isolated native fixture manager required", err, ce)
	}
	binary := sourcePreflightCommand(t)
	r, request := sourcePreflightFixture(t)
	dir, pin, _ := infrastructureDistribution(t, binary, nil, 1, nil, []byte(infrastructure.SourceTemplate))
	history := filepath.Join(filepath.Dir(dir), "history")
	program := filepath.Join(dir, "ardents-node-linux-amd64")
	run := func(args []string, want bool) {
		t.Helper()
		out, err := exec.CommandContext(t.Context(), program, args...).CombinedOutput()
		if (err == nil) != want {
			if want {
				diagnoseSourcePreflight(t, dir, history, request)
			}
			t.Fatalf("isolated original consumer success=%v: %s %v", want, out, err)
		}
	}
	run([]string{"infrastructure", "preflight-initial", dir, pin, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339), request}, true)
	args := []string{"infrastructure", "preflight-candidate", dir, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339), request}
	for _, fixture := range []struct{ path, line string }{
		{"/etc/passwd", "ardents-source:x:23456:23456:foreign fixture:/nonexistent:/usr/sbin/nologin\n"},
		{"/etc/group", "ardents-source:x:23456:\n"},
	} {
		t.Run(filepath.Base(fixture.path), func(t *testing.T) {
			before, err := os.ReadFile(fixture.path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.WriteFile(fixture.path, before, 0644); err != nil {
					t.Error("original fixture namespace restore", err)
				}
			}()
			if err := os.WriteFile(fixture.path, append(bytes.Clone(before), []byte(fixture.line)...), 0644); err != nil {
				t.Fatal(err)
			}
			run(args, false)
		})
		run(args, true)
	}
	for _, name := range []string{"ardents-source.service", "ardents-source.service.d", "ardents-source.socket", "ardents-source.socket.d"} {
		path := filepath.Join("/etc/systemd/system", name)
		if strings.HasSuffix(name, ".d") {
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		} else {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte("[Service]\nExecStart=/usr/bin/false\n")
			if strings.HasSuffix(name, ".socket") {
				raw = []byte("[Socket]\nListenStream=127.0.0.1:4567\n")
			}
			_, err = f.Write(raw)
			if err = errors.Join(err, f.Close()); err != nil {
				t.Fatal(err)
			}
		}
		run(args, false)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		run(args, true)
	}
	for _, hard := range []bool{false, true} {
		alias := filepath.Join(filepath.Dir(request), "alias")
		if hard {
			err = os.Link(r.PrivateKey, alias)
		} else {
			err = os.Symlink(r.PrivateKey, alias)
		}
		if err != nil {
			t.Fatal(err)
		}
		copy := r
		copy.PrivateKey = alias
		writeSourcePreflightRequest(t, request, copy)
		run(args, false)
		if err = os.Remove(alias); err != nil {
			t.Fatal(err)
		}
		writeSourcePreflightRequest(t, request, r)
		run(args, true)
	}
	for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.CredentialDirectory} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("isolated preflight created destination", err)
		}
	}
}
