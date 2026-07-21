package readiness

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type NetworkProber struct{}

func (NetworkProber) Check(ctx context.Context, endpoint string, generation int64, timeout time.Duration) CheckResult {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.User != nil {
		return failedCheck(ReasonInvalidEndpoint)
	}
	switch parsed.Scheme {
	case "http", "https":
		if !localProbeHost(parsed.Hostname()) || parsed.Fragment != "" {
			return failedCheck(ReasonInvalidEndpoint)
		}
		return checkHTTP(ctx, parsed, generation, timeout)
	case "tcp":
		if !localProbeHost(parsed.Hostname()) || parsed.Port() == "" {
			return failedCheck(ReasonInvalidEndpoint)
		}
		return checkStream(ctx, "tcp", parsed.Host, generation, timeout)
	case "unix":
		if !validUnixProbePath(parsed.Path) {
			return failedCheck(ReasonInvalidEndpoint)
		}
		return checkStream(ctx, "unix", parsed.Path, generation, timeout)
	default:
		return failedCheck(ReasonUnsupportedScheme)
	}
}

func localProbeHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validUnixProbePath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && !strings.ContainsRune(path, '\x00')
}

func checkHTTP(parent context.Context, endpoint *url.URL, generation int64, timeout time.Duration) CheckResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return failedCheck(ReasonInvalidEndpoint)
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: rejectRedirect}
	resp, err := client.Do(req)
	if err != nil {
		return failedNetworkCheck(ctx)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return closeHTTPResponse(resp, failedCheck(ReasonListenerUnreachable))
	}
	if resp.Header.Get("X-Ardents-Generation") != strconv.FormatInt(generation, 10) {
		return closeHTTPResponse(resp, failedCheck(ReasonGenerationMismatch))
	}
	return closeHTTPResponse(resp, CheckResult{Reachable: true, Reason: ReasonReady})
}

func closeHTTPResponse(response *http.Response, result CheckResult) CheckResult {
	if err := response.Body.Close(); err != nil {
		return failedCheck(ReasonListenerUnreachable)
	}
	return result
}

func rejectRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func checkStream(parent context.Context, network, address string, generation int64, timeout time.Duration) CheckResult {
	if address == "" {
		return failedCheck(ReasonInvalidEndpoint)
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return failedNetworkCheck(ctx)
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return closeProbeConnection(conn, failedCheck(ReasonListenerUnreachable))
	}
	challenge := fmt.Sprintf("ARDENTS READY %d\n", generation)
	if _, err := conn.Write([]byte(challenge)); err != nil {
		return closeProbeConnection(conn, failedCheck(ReasonListenerUnreachable))
	}
	reader := bufio.NewReaderSize(conn, 128)
	response, err := reader.ReadSlice('\n')
	if err != nil || len(response) > 128 {
		return closeProbeConnection(conn, failedCheck(ReasonListenerUnreachable))
	}
	if string(response) != challenge {
		return closeProbeConnection(conn, failedCheck(ReasonGenerationMismatch))
	}
	return closeProbeConnection(conn, CheckResult{Reachable: true, Reason: ReasonReady})
}

func closeProbeConnection(connection net.Conn, result CheckResult) CheckResult {
	if err := connection.Close(); err != nil {
		return failedCheck(ReasonListenerUnreachable)
	}
	return result
}

func failedNetworkCheck(ctx context.Context) CheckResult {
	if ctx.Err() != nil {
		return failedCheck(ReasonProbeTimeout)
	}
	return failedCheck(ReasonListenerUnreachable)
}

func failedCheck(reason string) CheckResult {
	return CheckResult{Reason: reason}
}
