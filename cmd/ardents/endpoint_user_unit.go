package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// runEndpointUserUnit writes the explicit Ubuntu systemd --user unit for one
// exact Portable artifact, bundle directory, and manifest pin. It does not write the unit,
// reload systemd, enable it, or start the Endpoint.
func runEndpointUserUnit(bundleRoot, manifestSHA256 string, output io.Writer) error {
	if runtime.GOOS != "linux" {
		return errors.New("endpoint user unit is available only on Linux")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	bundleRoot, err = filepath.Abs(bundleRoot)
	if err != nil {
		return err
	}
	unit, err := portableEnrollmentUserUnit(executable, bundleRoot, manifestSHA256, "Ardents Portable Endpoint (closed alpha)")
	if err != nil {
		return err
	}
	_, err = io.WriteString(output, unit)
	return err
}

// runLegacyEndpointUserUnit turns one historical Portable enrollment input
// into the current pin-argument unit. It exists only for explicit migration;
// new units never retain the JSON path.
func runLegacyEndpointUserUnit(path string, output io.Writer) error {
	input, err := loadLegacyPortableEnrollment(path)
	if err != nil {
		return err
	}
	return runEndpointUserUnit(input.BundleRoot, input.ManifestSHA256, output)
}

func portableEnrollmentUserUnit(executable, bundleRoot, manifestSHA256, description string) (string, error) {
	if description == "" {
		return "", errors.New("portable Endpoint user unit description is required")
	}
	executable, err := unitArgument(executable)
	if err != nil {
		return "", fmt.Errorf("portable Endpoint executable: %w", err)
	}
	bundleRoot, err = unitArgument(bundleRoot)
	if err != nil {
		return "", fmt.Errorf("portable Endpoint bundle root: %w", err)
	}
	manifestSHA256, err = manifestPinArgument(manifestSHA256)
	if err != nil {
		return "", err
	}
	return "[Unit]\n" +
		"Description=" + description + "\n" +
		"After=default.target\n\n" +
		"[Service]\n" +
		"Type=simple\n" +
		"UMask=0077\n" +
		"Restart=no\n" +
		"ExecStart=" + executable + " endpoint enroll " + bundleRoot + " " + manifestSHA256 + "\n\n" +
		"[Install]\n" +
		"WantedBy=default.target\n", nil
}

// runEndpointInstalledUserUnit renders the explicit per-user service for one
// already-installed Ubuntu package enrollment. It neither invokes dpkg nor
// enables, starts, lingers, or administers a user service.
func runEndpointInstalledUserUnit(input string, output io.Writer) error {
	return runEndpointInstalledEnrollmentUserUnit(input, "Ardents Installed Endpoint (closed alpha)", output)
}

func runEndpointInstalledEnrollmentUserUnit(input, description string, output io.Writer) error {
	if runtime.GOOS != "linux" {
		return errors.New("endpoint user unit is available only on Linux")
	}
	if input == "" {
		return errors.New("endpoint enrollment input is required")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	enrollment, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	unit, err := installedEnrollmentUserUnit(executable, enrollment, description)
	if err != nil {
		return err
	}
	_, err = io.WriteString(output, unit)
	return err
}

func installedEnrollmentUserUnit(executable, enrollment, description string) (string, error) {
	if description == "" {
		return "", errors.New("installed Endpoint user unit description is required")
	}
	executable, err := unitArgument(executable)
	if err != nil {
		return "", fmt.Errorf("portable Endpoint executable: %w", err)
	}
	enrollment, err = unitArgument(enrollment)
	if err != nil {
		return "", fmt.Errorf("portable Endpoint enrollment input: %w", err)
	}
	return "[Unit]\n" +
		"Description=" + description + "\n" +
		"After=default.target\n\n" +
		"[Service]\n" +
		"Type=simple\n" +
		"UMask=0077\n" +
		"Restart=no\n" +
		"ExecStart=" + executable + " endpoint enroll-installed " + enrollment + "\n\n" +
		"[Install]\n" +
		"WantedBy=default.target\n", nil
}

// unitArgument produces one systemd ExecStart argument. It rejects controls
// and escapes the syntax that systemd expands before it executes the process.
func unitArgument(value string) (string, error) {
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", errors.New("path must be clean and absolute")
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("path contains a control character")
	}
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "$$", "%", "%%")
	return "\"" + replacer.Replace(value) + "\"", nil
}

func manifestPinArgument(value string) (string, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 || strings.ToLower(value) != value {
		return "", errors.New("manifest pin must be 64 lowercase hexadecimal characters")
	}
	return value, nil
}
