//go:build browsercompat

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

func exerciseDynamicInput(input config, client *http.Client, origin string) (*dynamicWorkloadResult, error) {
	if input.DynamicWorkload.configured() {
		return exerciseConfiguredDynamic(client, origin, input.DynamicWorkload.plan(), input.PublisherTerminal, input.TransitFault)
	}
	var err error
	switch input.PublisherTerminal {
	case publisherTerminalEndpointStop:
		err = exerciseDynamicPublisherEndpointCrash(client, origin)
	case publisherTerminalApplicationReset:
		err = exerciseDynamicApplicationCrash(client, origin)
	default:
		err = exerciseDynamicReference(client, origin)
	}
	return nil, err
}

func exerciseConfiguredDynamic(client *http.Client, origin string, plan dynamicWorkloadPlan,
	terminal publisherTerminal, fault transitFault,
) (*dynamicWorkloadResult, error) {
	result := newDynamicWorkloadResult(plan)
	if client == nil || plan.cycles == 0 || plan.interval <= 0 || plan.cycleDeadline <= 0 {
		return &result, errors.New("configured dynamic User workload is invalid")
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	// The first cycle occupies the first interval instead of being emitted at
	// t=0, so 1,800 cycles at one cycle/second are a full 30-minute soak.
	workloadStarted := time.Now()
	firstStart := workloadStarted.Add(plan.interval)
	result.PlannedStartAtUTC = firstStart.UTC()
	for cycle := uint32(1); cycle <= plan.cycles; cycle++ {
		scheduled := firstStart.Add(time.Duration(cycle-1) * plan.interval)
		if err := waitForDynamicCycle(scheduled); err != nil {
			return &result, err
		}
		started := time.Now()
		if cycle == 1 {
			result.ActualStartAtUTC = started.UTC()
		}
		startLag := started.Sub(scheduled)
		startLagLimit := plan.startLagLimit()
		if missedDynamicPacingSlot(startLag, startLagLimit) {
			return &result, fmt.Errorf("configured dynamic cycle %d missed a pacing slot: start lag %s reached %s limit; previous maximum cycle latency %dµs",
				cycle, startLag.Round(time.Microsecond), startLagLimit, result.MaximumCycleLatencyMicros)
		}
		ctx, cancel := context.WithTimeout(context.Background(), plan.cycleDeadline)
		err := exerciseConfiguredDynamicCycle(ctx, client, origin, cycle)
		cancel()
		if err != nil {
			return &result, fmt.Errorf("configured dynamic cycle %d: %w", cycle, err)
		}
		result.recordCycle(time.Since(started), startLag)
		if plan.noFallbackEvery != 0 && cycle%plan.noFallbackEvery == 0 {
			ctx, cancel := context.WithTimeout(context.Background(), plan.cycleDeadline)
			err := probeConfiguredDynamicUnselected(ctx, client)
			cancel()
			if err != nil {
				return &result, fmt.Errorf("configured dynamic no-fallback round after cycle %d: %w", cycle, err)
			}
			result.PeriodicNoFallbackProbeRounds++
		}
	}
	result.ElapsedMicros = max(int64(1), time.Since(workloadStarted).Microseconds())
	result.finalizeLatencyQuantiles()
	if fault != "" {
		return exerciseConfiguredDynamicTransitFault(client, origin, plan, &result, fault)
	}
	switch terminal {
	case publisherTerminalApplicationReset:
		return exerciseConfiguredDynamicApplicationLoss(client, origin, plan, &result)
	case publisherTerminalEndpointStop:
		return exerciseConfiguredDynamicEndpointLoss(client, origin, plan, &result)
	default:
		return &result, closeConfiguredDynamic(client, origin, plan)
	}
}

func exerciseConfiguredDynamicCycle(ctx context.Context, client *http.Client, origin string, cycle uint32) error {
	response, err := beginConfiguredDynamicPublishAndTimeline(ctx, client, origin, cycle)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/plain" {
		return errors.New("configured dynamic Publisher response headers were not preserved")
	}
	first := make([]byte, len("first-"))
	if _, err := io.ReadFull(response.Body, first); err != nil || string(first) != "first-" {
		return errors.New("configured dynamic Publisher first response chunk was not preserved")
	}
	rest, err := io.ReadAll(response.Body)
	if err != nil || string(rest) != "second" {
		return errors.New("configured dynamic Publisher streamed response was not preserved")
	}
	return nil
}

func beginConfiguredDynamicPublishAndTimeline(ctx context.Context, client *http.Client, origin string, cycle uint32) (*http.Response, error) {
	post, err := http.NewRequestWithContext(ctx, http.MethodPost, origin+"publish?draft=1", bytes.NewBufferString("title=ardents"))
	if err != nil {
		return nil, err
	}
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Cookie", "before=bridge")
	post.Header.Set(dynamicWorkloadCycleHeader, strconv.FormatUint(uint64(cycle), 10))
	response, err := client.Do(post)
	if err != nil {
		return nil, err
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "/timeline" ||
		response.Header.Get("Set-Cookie") != "session=dynamic; Path=/" {
		return nil, fmt.Errorf("configured dynamic Publisher redirect or cookie was not preserved: status=%d location=%q set-cookie=%q",
			response.StatusCode, response.Header.Get("Location"), response.Header.Get("Set-Cookie"))
	}
	get, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"timeline", nil)
	if err != nil {
		return nil, err
	}
	get.Header.Set("Cookie", "session=dynamic")
	get.Header.Set(dynamicWorkloadCycleHeader, strconv.FormatUint(uint64(cycle), 10))
	return client.Do(get)
}

func closeConfiguredDynamic(client *http.Client, origin string, plan dynamicWorkloadPlan) error {
	ctx, cancel := context.WithTimeout(context.Background(), plan.cycleDeadline)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"close", nil)
	if err != nil {
		return err
	}
	request.Header.Set(dynamicWorkloadCycleHeader, strconv.FormatUint(uint64(plan.cycles), 10))
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return errors.New("configured dynamic Publisher did not close after its final cycle")
	}
	return nil
}

func exerciseConfiguredDynamicApplicationLoss(client *http.Client, origin string, plan dynamicWorkloadPlan,
	result *dynamicWorkloadResult,
) (*dynamicWorkloadResult, error) {
	terminalStarted := time.Now()
	terminalDeadline := terminalStarted.Add(15 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), dynamicTerminalRequestDeadline(terminalStarted, terminalDeadline, plan.cycleDeadline))
	response, err := beginConfiguredDynamicPublishAndTimeline(ctx, client, origin, plan.cycles+1)
	if err != nil {
		cancel()
		return result, err
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/plain" {
		_ = response.Body.Close()
		cancel()
		return result, errors.New("configured dynamic Publisher partial response headers were not preserved")
	}
	partial, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	cancel()
	if string(partial) != "first-" {
		return result, errors.New("configured dynamic Publisher Application loss did not stop at the declared partial response")
	}
	if err := requireConfiguredDynamicNoFallback(client, origin, terminalDeadline); err != nil {
		return result, err
	}
	if time.Now().After(terminalDeadline) {
		return result, errors.New("publisher Application loss exceeded the 15-second terminal bound")
	}
	result.TerminalNoFallback = true
	result.TerminalLatencyMicros = max(int64(1), time.Since(terminalStarted).Microseconds())
	return result, nil
}

func exerciseConfiguredDynamicEndpointLoss(client *http.Client, origin string, plan dynamicWorkloadPlan,
	result *dynamicWorkloadResult,
) (*dynamicWorkloadResult, error) {
	terminalStarted := time.Now()
	terminalDeadline := terminalStarted.Add(15 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), dynamicTerminalRequestDeadline(terminalStarted, terminalDeadline, plan.cycleDeadline))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"crash", nil)
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
			return result, errors.New("hard-stopped Publisher Endpoint reported HTTP success")
		}
	}
	deadlineExceeded := errors.Is(ctx.Err(), context.DeadlineExceeded)
	cancel()
	if deadlineExceeded {
		return result, errors.New("publisher Endpoint was not hard-stopped within the configured cycle deadline")
	}
	if requestErr == nil && response == nil {
		return result, errors.New("publisher Endpoint loss produced no terminal observable")
	}
	if err := requireConfiguredDynamicNoFallback(client, origin, terminalDeadline); err != nil {
		return result, err
	}
	if time.Now().After(terminalDeadline) {
		return result, errors.New("publisher Endpoint loss exceeded the 15-second terminal bound")
	}
	result.TerminalNoFallback = true
	result.TerminalLatencyMicros = max(int64(1), time.Since(terminalStarted).Microseconds())
	return result, nil
}

func probeConfiguredDynamicUnselected(ctx context.Context, client *http.Client) error {
	for _, probe := range []struct {
		url    string
		status int
	}{{"http://unregistered.ard/", http.StatusNotFound}, {"http://ordinary.invalid/", http.StatusBadRequest}} {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.url, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != probe.status {
			return errors.New("configured dynamic Browser Entry forwarded an unselected destination")
		}
	}
	return nil
}

func requireConfiguredDynamicNoFallback(client *http.Client, origin string, deadline time.Time) error {
	for _, candidate := range []string{origin, "http://unregistered.ard/", "http://ordinary.invalid/"} {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			cancel()
			return err
		}
		response, requestErr := client.Do(request)
		if response != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode < http.StatusBadRequest {
				cancel()
				return errors.New("configured dynamic failure selected a fallback destination")
			}
		}
		cancel()
		if requestErr == nil && response == nil {
			return errors.New("configured dynamic no-fallback probe produced no observable")
		}
	}
	return nil
}

func waitForDynamicCycle(scheduled time.Time) error {
	delay := time.Until(scheduled)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	<-timer.C
	return nil
}

func missedDynamicPacingSlot(startLag, limit time.Duration) bool {
	return limit <= 0 || startLag >= limit
}

func dynamicTerminalRequestDeadline(started, terminal time.Time, cycleDeadline time.Duration) time.Time {
	request := started.Add(cycleDeadline)
	if terminal.Before(request) {
		return terminal
	}
	return request
}
