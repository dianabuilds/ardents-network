package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	"github.com/dianabuilds/ardents-network/internal/browserentry/installer"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
)

// installBrowserEntry verifies the selected enrollment-v4 bundle before it
// writes the one fixed per-user native-manifest registration. It deliberately
// does not install or open the signed XPI: Firefox must perform that explicit
// participant action and validate Mozilla's signature itself.
func installBrowserEntry(arguments []string, output io.Writer) error {
	return installBrowserEntryWith(arguments, output, enrollment.VerifyBrowser, enrollment.VerifyRunningCompanion, installer.Install)
}

func installBrowserEntryWith(arguments []string, output io.Writer,
	verify func(enrollment.Request) (enrollment.Verified, error),
	verifyCompanion func(enrollment.Request, string, []byte) error,
	install func(string, string) (installer.Result, error)) error {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var enrollmentPath, endpointArtifact, atText string
	flags.StringVar(&enrollmentPath, "enrollment", "", "alpha enrollment input JSON")
	flags.StringVar(&endpointArtifact, "endpoint-artifact", "", "exact enrolled Endpoint artifact path")
	flags.StringVar(&atText, "at", "", "decision time in RFC3339")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || endpointArtifact == "" {
		return errors.New("browser Entry installation arguments are invalid")
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return errors.New("browser Entry installation time is invalid")
	}
	input, err := enrollment.ReadClosedAlphaInput(enrollmentPath)
	if err != nil {
		return err
	}
	request := input.Request(endpointArtifact, at)
	verified, err := verify(request)
	if err != nil {
		return err
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	if request.Pin.Platform != platform || verified.BrowserEntryArtifactName != browserentry.HostArtifactName(platform) ||
		verified.BrowserEntryExtensionName != browserentry.ExtensionArtifactName || len(verified.BrowserEntryArtifact) == 0 || len(verified.BrowserEntryExtension) == 0 {
		return errors.New("browser Entry installation requires an enrolled v4 Browser Entry profile")
	}
	if err := verifyCompanion(request, verified.BrowserEntryArtifactName, verified.BrowserEntryArtifact); err != nil {
		return errors.New("running Browser Entry host is not the enrolled Browser Entry artifact")
	}
	bundle, err := filepath.Abs(request.BundleRoot)
	if err != nil {
		return errors.New("resolve enrolled Browser Entry bundle")
	}
	installed, err := install(filepath.Join(bundle, verified.BrowserEntryArtifactName), filepath.Join(bundle, verified.BrowserEntryExtensionName))
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema                string `json:"schema"`
		NativeManifest        string `json:"native_manifest"`
		Extension             string `json:"extension"`
		ExtensionInstallation string `json:"extension_installation"`
	}{Schema: "ardents-browser-entry-installation-v1", NativeManifest: installed.NativeManifestPath, Extension: installed.ExtensionPath,
		ExtensionInstallation: "manual-required"})
}

// removeBrowserEntry withdraws only the fixed native manifest. It cannot
// uninstall the signed XPI or remove Endpoint, authority, or corpus data.
func removeBrowserEntry(arguments []string, output io.Writer) error {
	if len(arguments) != 0 {
		return errors.New("browser Entry removal arguments are invalid")
	}
	if err := installer.Remove(); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema  string `json:"schema"`
		Removal string `json:"removal"`
	}{Schema: "ardents-browser-entry-removal-v1", Removal: "native-manifest-withdrawn"})
}
