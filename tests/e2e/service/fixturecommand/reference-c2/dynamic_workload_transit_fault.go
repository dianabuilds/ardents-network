//go:build referencec2

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

func serveConfiguredDynamicTransitFault(connection net.Conn, reader *bufio.Reader, proofPath, readyPath string,
	plan dynamicWorkloadPlan, result dynamicWorkloadResult, fault transitFault,
) (dynamicWorkloadResult, error) {
	if !validTransitFault(fault) || readyPath == "" {
		return result, errors.New("configured transit fault control is unavailable")
	}
	if err := connection.SetDeadline(time.Now().Add(plan.cycleDeadline)); err != nil {
		return result, err
	}
	request, err := http.ReadRequest(reader)
	if err != nil {
		return result, err
	}
	_ = request.Body.Close()
	if request.Method != http.MethodGet || request.Host != "reference.ard" || request.URL.String() != "/fault" ||
		request.Header.Get(dynamicWorkloadCycleHeader) != strconv.FormatUint(uint64(plan.cycles+1), 10) {
		return result, errors.New("configured transit fault request was changed, replayed, or incomplete")
	}
	if err := connection.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return result, err
	}
	if err := os.WriteFile(proofPath, []byte("dynamic-"+string(fault)+"\n"), 0o600); err != nil {
		return result, err
	}
	if err := os.WriteFile(readyPath, []byte("ready\n"), 0o600); err != nil {
		return result, err
	}
	_, _ = io.Copy(io.Discard, reader)
	return result, fmt.Errorf("simulated %s closed the local Application handoff", transitFaultLabel(fault))
}

func exerciseConfiguredDynamicTransitFault(client *http.Client, origin string, plan dynamicWorkloadPlan,
	result *dynamicWorkloadResult, fault transitFault,
) (*dynamicWorkloadResult, error) {
	if !validTransitFault(fault) {
		return result, errors.New("configured transit fault is invalid")
	}
	terminalStarted := time.Now()
	terminalDeadline := terminalStarted.Add(15 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), dynamicTerminalRequestDeadline(terminalStarted, terminalDeadline, plan.cycleDeadline))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"fault", nil)
	if err != nil {
		cancel()
		return result, err
	}
	request.Header.Set(dynamicWorkloadCycleHeader, strconv.FormatUint(uint64(plan.cycles+1), 10))
	response, requestErr := client.Do(request)
	if response != nil {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode < http.StatusBadRequest {
			cancel()
			return result, fmt.Errorf("%s reported HTTP success", transitFaultLabel(fault))
		}
	}
	deadlineExceeded := errors.Is(ctx.Err(), context.DeadlineExceeded)
	cancel()
	if deadlineExceeded {
		return result, fmt.Errorf("%s was not observed within the configured cycle deadline", transitFaultLabel(fault))
	}
	if requestErr == nil && response == nil {
		return result, errors.New("transit fault produced no terminal observable")
	}
	if err := requireConfiguredDynamicNoFallback(client, origin, terminalDeadline); err != nil {
		return result, err
	}
	if time.Now().After(terminalDeadline) {
		return result, fmt.Errorf("%s exceeded the 15-second terminal bound", transitFaultLabel(fault))
	}
	result.TerminalNoFallback = true
	result.TerminalLatencyMicros = max(int64(1), time.Since(terminalStarted).Microseconds())
	return result, nil
}

func validTransitFault(fault transitFault) bool {
	return fault == transitFaultCarrierLoss || fault == transitFaultProductNodeLoss
}

func transitFaultLabel(fault transitFault) string {
	if fault == transitFaultProductNodeLoss {
		return "product Node loss"
	}
	return "Carrier loss"
}
