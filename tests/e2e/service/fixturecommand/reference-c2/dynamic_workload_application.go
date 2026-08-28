package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

const dynamicWorkloadCycleHeader = "X-Ardents-Qualification-Cycle"

func serveConfiguredDynamic(connection net.Conn, proofPath string, controls dynamicWorkloadControls, plan dynamicWorkloadPlan,
	terminal publisherTerminal, fault transitFault,
) (dynamicWorkloadResult, error) {
	result := newDynamicWorkloadResult(plan)
	if connection == nil || plan.cycles == 0 || plan.interval <= 0 || plan.cycleDeadline <= 0 {
		return result, errors.New("configured dynamic Publisher workload is invalid")
	}
	defer connection.Close()
	reader := bufio.NewReader(connection)
	for cycle := uint32(1); cycle <= plan.cycles; cycle++ {
		started := time.Now()
		deadline := plan.cycleDeadline
		if cycle == 1 {
			deadline = plan.initialRequestDeadline()
		}
		if err := connection.SetDeadline(started.Add(deadline)); err != nil {
			return result, err
		}
		if err := acceptDynamicPublishAndTimelineCycle(connection, reader, cycle); err != nil {
			return result, err
		}
		if err := writeConfiguredDynamicTimeline(connection); err != nil {
			return result, err
		}
		result.recordCycle(time.Since(started), 0)
	}
	result.finalizeLatencyQuantiles()
	if fault != "" {
		return serveConfiguredDynamicTransitFault(connection, reader, proofPath, controls.transitFaultReady, plan, result, fault)
	}
	switch terminal {
	case publisherTerminalApplicationReset:
		return serveConfiguredDynamicApplicationLoss(connection, reader, proofPath, controls.applicationFaultReady, controls.applicationFaultRelease, plan, result)
	case publisherTerminalEndpointStop:
		return serveConfiguredDynamicEndpointLoss(connection, reader, proofPath, controls.endpointCrashReady, plan, result)
	default:
		return serveConfiguredDynamicClose(connection, reader, proofPath, plan, result)
	}
}

func serveConfiguredDynamicApplicationLoss(connection net.Conn, reader *bufio.Reader, proofPath, readyPath, releasePath string, plan dynamicWorkloadPlan,
	result dynamicWorkloadResult,
) (dynamicWorkloadResult, error) {
	if err := connection.SetDeadline(time.Now().Add(plan.cycleDeadline)); err != nil {
		return result, err
	}
	if err := acceptDynamicPublishAndTimelineCycle(connection, reader, plan.cycles+1); err != nil {
		return result, err
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfirst-\r\n"); err != nil {
		return result, err
	}
	if err := os.WriteFile(proofPath, []byte("dynamic-application-crash\n"), 0o600); err != nil {
		return result, err
	}
	if readyPath != "" || releasePath != "" {
		if err := waitForConfiguredApplicationFault(readyPath, releasePath, plan.cycleDeadline); err != nil {
			return result, err
		}
	}
	if tcp, ok := connection.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = connection.Close()
	return result, errors.New("simulated Publisher Application crash after configured warmup")
}

func serveConfiguredDynamicEndpointLoss(connection net.Conn, reader *bufio.Reader, proofPath, crashReadyPath string,
	plan dynamicWorkloadPlan, result dynamicWorkloadResult,
) (dynamicWorkloadResult, error) {
	if crashReadyPath == "" {
		return result, errors.New("configured Publisher Endpoint loss control is unavailable")
	}
	if err := connection.SetDeadline(time.Now().Add(plan.cycleDeadline)); err != nil {
		return result, err
	}
	request, err := http.ReadRequest(reader)
	if err != nil {
		return result, err
	}
	_ = request.Body.Close()
	if request.Method != http.MethodGet || request.Host != "reference.ard" || request.URL.String() != "/crash" ||
		request.Header.Get(dynamicWorkloadCycleHeader) != strconv.FormatUint(uint64(plan.cycles+1), 10) {
		return result, errors.New("configured Publisher Endpoint loss request was changed, replayed, or incomplete")
	}
	if err := os.WriteFile(proofPath, []byte("dynamic-endpoint-crash\n"), 0o600); err != nil {
		return result, err
	}
	if err := connection.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return result, err
	}
	if err := os.WriteFile(crashReadyPath, []byte("ready\n"), 0o600); err != nil {
		return result, err
	}
	_, _ = io.Copy(io.Discard, reader)
	return result, errors.New("simulated Publisher Endpoint crash closed the local Application handoff")
}

func serveConfiguredDynamicClose(connection net.Conn, reader *bufio.Reader, proofPath string, plan dynamicWorkloadPlan,
	result dynamicWorkloadResult,
) (dynamicWorkloadResult, error) {
	if err := connection.SetDeadline(time.Now().Add(plan.cycleDeadline)); err != nil {
		return result, err
	}
	request, err := http.ReadRequest(reader)
	if err != nil {
		return result, err
	}
	_ = request.Body.Close()
	if request.Method != http.MethodGet || request.Host != "reference.ard" || request.URL.String() != "/close" ||
		request.Header.Get(dynamicWorkloadCycleHeader) != strconv.FormatUint(uint64(plan.cycles), 10) {
		return result, errors.New("configured dynamic Publisher close request was changed, replayed, or incomplete")
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n"); err != nil {
		return result, err
	}
	if err := os.WriteFile(proofPath, []byte("dynamic-http\n"), 0o600); err != nil {
		return result, err
	}
	return result, nil
}

func acceptDynamicPublishAndTimelineCycle(connection net.Conn, reader *bufio.Reader, cycle uint32) error {
	post, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(post.Body)
	_ = post.Body.Close()
	if err != nil || post.Method != http.MethodPost || post.Host != "reference.ard" || post.URL.String() != "/publish?draft=1" ||
		string(body) != "title=ardents" || post.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
		post.Header.Get("Cookie") != "before=bridge" || !dynamicWorkloadCycleMatches(post, cycle) {
		return errors.New("dynamic Publisher request was changed, replayed, or incomplete")
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 302 Found\r\nLocation: /timeline\r\nSet-Cookie: session=dynamic; Path=/\r\nContent-Length: 0\r\n\r\n"); err != nil {
		return err
	}
	get, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	_ = get.Body.Close()
	if get.Method != http.MethodGet || get.Host != "reference.ard" || get.URL.String() != "/timeline" ||
		get.Header.Get("Cookie") != "session=dynamic" || !dynamicWorkloadCycleMatches(get, cycle) {
		return errors.New("dynamic Publisher follow-up request was changed, replayed, or incomplete")
	}
	return nil
}

func dynamicWorkloadCycleMatches(request *http.Request, cycle uint32) bool {
	if cycle == 0 {
		return request.Header.Get(dynamicWorkloadCycleHeader) == ""
	}
	return request.Header.Get(dynamicWorkloadCycleHeader) == strconv.FormatUint(uint64(cycle), 10)
}

func writeConfiguredDynamicTimeline(connection net.Conn) error {
	if _, err := fmt.Fprint(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfirst-\r\n"); err != nil {
		return err
	}
	time.Sleep(25 * time.Millisecond)
	_, err := fmt.Fprint(connection, "6\r\nsecond\r\n0\r\n\r\n")
	return err
}
