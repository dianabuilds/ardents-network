package camouflage

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

const (
	maximumControlLine       = 4096
	maximumControlLines      = 64
	maximumControlTranscript = 64 << 10
)

func readClientReadiness(input io.Reader) (string, []byte, error) {
	version, endpoint := false, false
	return scanReadiness(input, func(fields []string) (string, bool, error) {
		switch fields[0] {
		case "VERSION":
			if version || endpoint || len(fields) != 2 || fields[1] != "1" {
				return "", false, errors.New("invalid VERSION")
			}
			version = true
		case "CMETHOD":
			if !version || endpoint || len(fields) != 4 || fields[1] != "webtunnel" || fields[2] != "socks5" {
				return "", false, errors.New("invalid CMETHOD")
			}
			host, port, err := net.SplitHostPort(fields[3])
			ip := net.ParseIP(host)
			parsedPort, portErr := strconv.ParseUint(port, 10, 16)
			if err != nil || portErr != nil || parsedPort == 0 || strconv.FormatUint(parsedPort, 10) != port ||
				ip == nil || !ip.IsLoopback() {
				return "", false, errors.New("invalid client listener")
			}
			endpoint = true
		case "CMETHODS":
			if !version || !endpoint || len(fields) != 2 || fields[1] != "DONE" {
				return "", false, errors.New("invalid CMETHODS DONE")
			}
			return "", true, nil
		}
		if err := terminalControl(fields[0]); err != nil {
			return "", false, err
		}
		return "", false, nil
	})
}

func readServerReadiness(input io.Reader) (string, []byte, error) {
	version, endpoint := false, false
	return scanReadiness(input, func(fields []string) (string, bool, error) {
		switch fields[0] {
		case "VERSION":
			if version || endpoint || len(fields) != 2 || fields[1] != "1" {
				return "", false, errors.New("invalid VERSION")
			}
			version = true
		case "SMETHOD":
			if !version || endpoint || len(fields) < 3 || fields[1] != "webtunnel" {
				return "", false, errors.New("invalid SMETHOD")
			}
			endpoint = true
		case "SMETHODS":
			if !version || !endpoint || len(fields) != 2 || fields[1] != "DONE" {
				return "", false, errors.New("invalid SMETHODS DONE")
			}
			return "ready", true, nil
		}
		if err := terminalControl(fields[0]); err != nil {
			return "", false, err
		}
		return "", false, nil
	})
}

type readinessLine func([]string) (string, bool, error)

func scanReadiness(input io.Reader, accept readinessLine) (string, []byte, error) {
	scanner := bufio.NewScanner(io.LimitReader(input, maximumControlTranscript+1))
	scanner.Buffer(make([]byte, 256), maximumControlLine+1)
	var transcript strings.Builder
	for lines := 0; scanner.Scan(); lines++ {
		line := scanner.Text()
		if lines >= maximumControlLines || len(line) > maximumControlLine || !asciiControlLine(line) {
			return "", []byte(transcript.String()), errors.New("control limit exceeded")
		}
		transcript.WriteString(line)
		transcript.WriteByte('\n')
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return "", []byte(transcript.String()), errors.New("empty control line")
		}
		address, done, err := accept(fields)
		if err != nil {
			return "", []byte(transcript.String()), err
		}
		if done {
			if address == "" {
				address = fieldsAddress(transcript.String())
			}
			return address, []byte(transcript.String()), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", []byte(transcript.String()), err
	}
	return "", []byte(transcript.String()), errors.New("EOF before readiness")
}

func terminalControl(keyword string) error {
	switch keyword {
	case "CMETHOD-ERROR", "SMETHOD-ERROR", "VERSION-ERROR", "ENV-ERROR", "PROXY-ERROR":
		return fmt.Errorf("terminal control line %s", keyword)
	default:
		return nil
	}
}

func fieldsAddress(transcript string) string {
	for _, line := range strings.Split(transcript, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 4 && fields[0] == "CMETHOD" {
			return fields[3]
		}
	}
	return ""
}

func asciiControlLine(value string) bool {
	for _, character := range []byte(value) {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	return true
}
