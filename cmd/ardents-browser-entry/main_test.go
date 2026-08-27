package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	"github.com/dianabuilds/ardents-network/internal/browserentry/installer"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
)

func TestNativeHostRejectsUnboundedOrUnsupportedCommandInput(t *testing.T) {
	if err := run([]string{"other"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("unsupported Browser Entry command was accepted")
	}
	if err := run([]string{"native-host", "--state", "relative.json"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("native host accepted an unavailable relative state path")
	}
}

func TestParticipantInstallAndRemovalRequireTheirExactBoundedInputs(t *testing.T) {
	if err := run([]string{"install"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("Browser Entry installation accepted missing enrollment and artifact inputs")
	}
	if err := run([]string{"remove", "unexpected"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("Browser Entry removal accepted unexpected input")
	}
}

func TestParticipantInstallUsesOnlyVerifiedV4CompanionsAndKeepsXPIManual(t *testing.T) {
	bundle := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "enrollment.json")
	platform := runtime.GOOS + "-" + runtime.GOARCH
	input := enrollment.ClosedAlphaInput{Schema: "ardents-alpha-enrollment-input-v1", BundleRoot: bundle, Cohort: "cohort-1", Release: "alpha-1",
		Platform: platform, ManifestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Environment: "alpha", Network: "network-1", TargetPath: "ardents/" + platform + "/endpoint"}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	hostName, extensionName := browserentry.HostArtifactName(platform), browserentry.ExtensionArtifactName
	calledVerify, calledCompanion, calledInstall := false, false, false
	var output bytes.Buffer
	err = installBrowserEntryWith([]string{"--enrollment", inputPath, "--endpoint-artifact", filepath.Join(bundle, "ardents"), "--at", "2026-08-26T00:00:00Z"}, &output,
		func(request enrollment.Request) (enrollment.Verified, error) {
			calledVerify = request.BundleRoot == bundle && request.Pin.Platform == platform
			return enrollment.Verified{BrowserEntryArtifactName: hostName, BrowserEntryArtifact: []byte("host"),
				BrowserEntryExtensionName: extensionName, BrowserEntryExtension: []byte("xpi")}, nil
		}, func(_ enrollment.Request, name string, bytes []byte) error {
			calledCompanion = name == hostName && string(bytes) == "host"
			return nil
		}, func(host, extension string) (installer.Result, error) {
			calledInstall = host == filepath.Join(bundle, hostName) && extension == filepath.Join(bundle, extensionName)
			return installer.Result{NativeManifestPath: filepath.Join(bundle, "manifest.json"), ExtensionPath: extension}, nil
		})
	if err != nil || !calledVerify || !calledCompanion || !calledInstall {
		t.Fatalf("Browser Entry installation: verify=%t companion=%t install=%t error=%v", calledVerify, calledCompanion, calledInstall, err)
	}
	var result struct {
		ExtensionInstallation string `json:"extension_installation"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.ExtensionInstallation != "manual-required" {
		t.Fatalf("Browser Entry installation result = %s, error = %v", output.String(), err)
	}
}

func TestNativeManifestInvocationUsesTheFixedPerUserStatePath(t *testing.T) {
	configHome := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", configHome)
	case "linux":
		t.Setenv("XDG_CONFIG_HOME", configHome)
	default:
		t.Skip("native Browser Entry manifest test supports Windows and Linux profiles")
	}
	statePath, err := nativeHostStatePath(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(statePath) {
		t.Fatalf("native manifest state path is not absolute: %q", statePath)
	}
	if filepath.Base(statePath) != "alpha-proxy.json" {
		t.Fatalf("native manifest state path = %q", statePath)
	}
	relative, err := filepath.Rel(configHome, statePath)
	if err != nil || relative == "." || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		t.Fatalf("native manifest state path escaped its test config home: %q", statePath)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	publisher, err := browserentry.OpenPublisher(statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	if err := publisher.Publish(port); err != nil {
		t.Fatal(err)
	}
	capability := publisher.Capability()
	probed := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		buffer := make([]byte, 512)
		_, _ = connection.Read(buffer)
		_, _ = connection.Write([]byte("HTTP/1.1 204 No Content\r\n" + browserentry.ProbeHeader + ": " + hex.EncodeToString(capability[:]) + "\r\nContent-Length: 0\r\n\r\n"))
		close(probed)
	}()
	request := []byte(`{"operation":"loopback-proxy-port"}`)
	var input bytes.Buffer
	if err := binary.Write(&input, binary.LittleEndian, uint32(len(request))); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Write(request); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(nil, &input, &output); err != nil {
		t.Fatal(err)
	}
	select {
	case <-probed:
	case <-time.After(time.Second):
		t.Fatal("native manifest invocation did not probe the fixed state port")
	}
	var length uint32
	if err := binary.Read(&output, binary.LittleEndian, &length); err != nil || length == 0 || length > 4096 {
		t.Fatalf("native manifest response length = %d, error = %v", length, err)
	}
	body := make([]byte, length)
	if _, err := output.Read(body); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Port uint16 `json:"port"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.Port != port {
		t.Fatalf("native manifest response = %+v, error = %v", response, err)
	}
}
