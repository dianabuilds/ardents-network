//go:build ignore

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

const (
	maxControlLine       = 4096
	maxControlLines      = 64
	maxControlTranscript = 64 << 10
)

type readiness struct {
	address string
	args    map[string]string
}

func readReadiness(r io.Reader, method, kind string) (readiness, []byte, error) {
	scanner := bufio.NewScanner(io.LimitReader(r, maxControlTranscript+1))
	scanner.Buffer(make([]byte, 256), maxControlLine+1)
	var transcript strings.Builder
	version, endpoint, done := false, false, false
	result := readiness{args: make(map[string]string)}
	for lines := 0; scanner.Scan(); lines++ {
		line := scanner.Text()
		if lines >= maxControlLines || len(line) > maxControlLine {
			return result, []byte(transcript.String()), errors.New("control limit exceeded")
		}
		transcript.WriteString(line + "\n")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return result, []byte(transcript.String()), errors.New("empty control line")
		}
		switch fields[0] {
		case "VERSION":
			if version || len(fields) != 2 || fields[1] != "1" || endpoint || done {
				return result, []byte(transcript.String()), errors.New("invalid VERSION")
			}
			version = true
		case kind:
			if !version || endpoint || done {
				return result, []byte(transcript.String()), fmt.Errorf("invalid %s order", kind)
			}
			if err := parseEndpoint(fields, method, kind, &result); err != nil {
				return result, []byte(transcript.String()), err
			}
			endpoint = true
		case kind + "S":
			if !version || !endpoint || done || len(fields) != 2 || fields[1] != "DONE" {
				return result, []byte(transcript.String()), errors.New("invalid DONE")
			}
			done = true
			return result, []byte(transcript.String()), nil
		case "CMETHOD-ERROR", "SMETHOD-ERROR", "VERSION-ERROR", "ENV-ERROR", "PROXY-ERROR":
			return result, []byte(transcript.String()), fmt.Errorf("terminal control line %s", fields[0])
		default:
			if !asciiKeyword(fields[0]) {
				return result, []byte(transcript.String()), errors.New("invalid control keyword")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return result, []byte(transcript.String()), err
	}
	return result, []byte(transcript.String()), errors.New("EOF before readiness")
}

func parseEndpoint(fields []string, method, kind string, out *readiness) error {
	minimum := 3
	addressIndex := 2
	if kind == "CMETHOD" {
		minimum, addressIndex = 4, 3
		if len(fields) < minimum || fields[2] != "socks5" {
			return errors.New("invalid SOCKS method")
		}
	}
	if len(fields) < minimum || fields[1] != method {
		return errors.New("wrong PT method")
	}
	host, port, err := net.SplitHostPort(fields[addressIndex])
	if err != nil || port == "0" || net.ParseIP(host) == nil {
		return errors.New("invalid PT address")
	}
	if kind == "CMETHOD" && !net.ParseIP(host).IsLoopback() {
		return errors.New("non-loopback client listener")
	}
	out.address = fields[addressIndex]
	for _, field := range fields[addressIndex+1:] {
		if !strings.HasPrefix(field, "ARGS:") {
			return errors.New("unexpected endpoint field")
		}
		for _, pair := range strings.Split(strings.TrimPrefix(field, "ARGS:"), ",") {
			key, value, ok := strings.Cut(pair, "=")
			if !ok || key == "" {
				return errors.New("malformed endpoint args")
			}
			out.args[key] = value
		}
	}
	return nil
}

func asciiKeyword(value string) bool {
	for _, b := range []byte(value) {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return value != ""
}
