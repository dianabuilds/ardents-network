package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// runEndpointUserUnit writes the explicit Ubuntu systemd --user unit for one
// exact Portable artifact and enrollment input. It does not write the unit,
// reload systemd, enable it, or start the Endpoint.
func runEndpointUserUnit(input string, output io.Writer) error {
	return runEndpointEnrollmentUserUnit(input, "enroll", "Ardents Portable Endpoint (closed alpha)", output)
}

// runEndpointInstalledUserUnit renders the explicit per-user service for one
// already-installed Ubuntu package enrollment. It neither invokes dpkg nor
// enables, starts, lingers, or administers a user service.
func runEndpointInstalledUserUnit(input string, output io.Writer) error {
	return runEndpointEnrollmentUserUnit(input, "enroll-installed", "Ardents Installed Endpoint (closed alpha)", output)
}

func runEndpointEnrollmentUserUnit(input, action, description string, output io.Writer) error {
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
	unit, err := enrollmentUserUnit(executable, enrollment, action, description)
	if err != nil {
		return err
	}
	_, err = io.WriteString(output, unit)
	return err
}

func enrollmentUserUnit(executable, enrollment, action, description string) (string, error) {
	if action != "enroll" && action != "enroll-installed" || description == "" {
		return "", errors.New("endpoint user unit action is invalid")
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
		"ExecStart=" + executable + " endpoint " + action + " " + enrollment + "\n\n" +
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
