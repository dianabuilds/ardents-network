//go:build referencec2

package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type referenceC2BrowserEntryQualification struct {
	firefox, addonSource, nativeHost, runner, signedXPI string
}

func prepareReferenceC2BrowserEntryQualification(t *testing.T, signedXPI bool) referenceC2BrowserEntryQualification {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Fatal("C2 dynamic Firefox Browser Entry qualification requires Windows")
	}
	firefox := os.Getenv("ARDENTS_REFERENCE_C2_FIREFOX")
	info, err := os.Stat(firefox)
	if firefox == "" || err != nil || !info.Mode().IsRegular() {
		t.Fatal("ARDENTS_REFERENCE_C2_FIREFOX must name an existing Firefox executable")
	}
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	repository := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	value := referenceC2BrowserEntryQualification{firefox: firefox,
		addonSource: filepath.Join(repository, "packaging", "firefox-alpha-browser-entry"),
		nativeHost:  buildProductCommand(t, "ardents-browser-entry"),
		runner:      filepath.Join(repository, "tests", "qualification", "h4-4a-firefox", "run-maintained-browser-entry.ps1")}
	if signedXPI {
		value.signedXPI = os.Getenv("ARDENTS_H4_4_SIGNED_FIREFOX_XPI")
		if info, err := os.Stat(value.signedXPI); value.signedXPI == "" || err != nil || !info.Mode().IsRegular() {
			t.Fatal("ARDENTS_H4_4_SIGNED_FIREFOX_XPI must name the selected Mozilla-signed XPI")
		}
	}
	for _, path := range []string{value.addonSource, value.nativeHost, value.runner} {
		if info, err := os.Stat(path); err != nil || (path != value.addonSource && !info.Mode().IsRegular()) {
			t.Fatalf("C2 Browser Entry qualification input %q is unavailable: %v", path, err)
		}
	}
	return value
}

func startReferenceC2BrowserEntryQualification(ctx context.Context, root string, input referenceC2BrowserEntryQualification, statePath, proofPath string) <-chan commandResult {
	if ctx == nil || input.firefox == "" || input.addonSource == "" || input.nativeHost == "" || input.runner == "" ||
		!filepath.IsAbs(statePath) || !filepath.IsAbs(proofPath) {
		result := make(chan commandResult, 1)
		result <- commandResult{err: errors.New("C2 Browser Entry qualification input is invalid")}
		return result
	}
	timeout := "30"
	if input.signedXPI != "" {
		timeout = "180"
	}
	arguments := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", input.runner,
		"-FirefoxPath", input.firefox, "-AddonSource", input.addonSource, "-NativeHostPath", input.nativeHost,
		"-NativeStatePath", statePath, "-DynamicProofPath", proofPath, "-TimeoutSeconds", timeout}
	if input.signedXPI != "" {
		arguments = append(arguments, "-SignedXPI", input.signedXPI)
	}
	return startCommand(ctx, root, "powershell.exe", arguments...)
}
