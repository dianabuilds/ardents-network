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
	if runtime.GOOS != "linux" {
		return errors.New("portable Endpoint user unit is available only on Linux")
	}
	if input == "" {
		return errors.New("portable Endpoint enrollment input is required")
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
	unit, err := portableUserUnit(executable, enrollment)
	if err != nil {
		return err
	}
	_, err = io.WriteString(output, unit)
	return err
}

func portableUserUnit(executable, enrollment string) (string, error) {
	executable, err := unitArgument(executable)
	if err != nil {
		return "", fmt.Errorf("portable Endpoint executable: %w", err)
	}
	enrollment, err = unitArgument(enrollment)
	if err != nil {
		return "", fmt.Errorf("portable Endpoint enrollment input: %w", err)
	}
	return "[Unit]\n" +
		"Description=Ardents Portable Endpoint (closed alpha)\n" +
		"After=default.target\n\n" +
		"[Service]\n" +
		"Type=simple\n" +
		"UMask=0077\n" +
		"Restart=no\n" +
		"ExecStart=" + executable + " endpoint enroll " + enrollment + "\n\n" +
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
