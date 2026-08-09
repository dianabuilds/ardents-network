package tooling

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

func capturedVersion(pattern *regexp.Regexp, output string) string {
	match := pattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func hashRegularFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular tool artifact", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func deleteShaping(run externalCommand, networkInterface string) error {
	output, err := run("/usr/sbin/tc", "qdisc", "del", "dev", networkInterface, "root")
	if err != nil {
		return fmt.Errorf("remove tc netem: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func exchangeToolingMarker(connection net.Conn, runID, role, expectedPeer string) error {
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	marker := toolingMarkerPrefix + runID + "/"
	payload := marker + role + "\n" + strings.Repeat("synthetic-tracer-payload/", 512)
	if _, err := io.WriteString(connection, payload); err != nil {
		return err
	}
	line, err := bufio.NewReaderSize(connection, 1024).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != marker+expectedPeer {
		return errors.New("synthetic tracer observed an unexpected peer")
	}
	return nil
}

func scanCaptureStderr(reader io.Reader, ready chan<- struct{}, finished chan<- string) {
	scanner := bufio.NewScanner(io.LimitReader(reader, 16*1024))
	var lines []string
	reported := false
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if !reported && strings.Contains(line, "listening on ") {
			ready <- struct{}{}
			reported = true
		}
	}
	finished <- strings.Join(lines, "\n")
}
