//go:build referencec2

package service_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func assertPublisherApplicationRejection(t *testing.T, publisher, application <-chan commandResult, readyPath string) {
	t.Helper()
	publisherResult := <-publisher
	applicationResult := <-application
	if publisherResult.err == nil || !strings.Contains(string(publisherResult.output), "handoff token is invalid") {
		t.Fatalf("Publisher accepted untrusted local Application: %v\n%s", publisherResult.err, publisherResult.output)
	}
	if applicationResult.err == nil || !strings.Contains(string(applicationResult.output), "static request is invalid or incomplete") {
		t.Fatalf("untrusted local Application reported success: %v\n%s", applicationResult.err, applicationResult.output)
	}
	if _, err := os.Stat(readyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("untrusted local Application produced Publisher readiness: %v", err)
	}
}

func assertC2TransitDrained(t *testing.T, transit map[string]<-chan commandResult, gateway <-chan commandResult) {
	t.Helper()
	for role, process := range transit {
		assertC2DrainedResult(t, role, <-process)
	}
	assertC2DrainedResult(t, "gateway", <-gateway)
}

func assertC2DrainedResult(t *testing.T, role string, process commandResult) {
	t.Helper()
	if process.err != nil {
		t.Fatalf("C2 %s did not drain after local Application refusal: %v\n%s", role, process.err, process.output)
	}
	var observed struct {
		Schema, Role, Class string
		Passed              bool
	}
	line := strings.TrimSpace(string(process.output))
	if index := strings.LastIndex(line, "\n"); index >= 0 {
		line = line[index+1:]
	}
	if err := json.Unmarshal([]byte(line), &observed); err != nil || observed.Schema != "ardents-e2e-reference-c2-result-v1" || observed.Role != role || !observed.Passed || observed.Class != "drained" {
		t.Fatalf("C2 %s drain result = %q / %+v / %v", role, process.output, observed, err)
	}
}

func referenceC2WaitForFile(ctx context.Context, path string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func referenceC2Hex(value [32]byte) string { return hex.EncodeToString(value[:]) }
