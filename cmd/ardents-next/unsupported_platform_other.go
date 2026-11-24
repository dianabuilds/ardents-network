//go:build !linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// unsupportedPlatformRefusal projects the explicit typed unavailable outcome
// for non-native command stubs (ADR-0129 §3). Each caller supplies its operation
// and report writer. The exit code is 1 for a written refusal, or 2 when the
// report cannot be written.
func unsupportedPlatformRefusal(out io.Writer, operation string) int {
	err := json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{operation, "refused", "unsupported-platform"})
	if err != nil {
		return 2
	}
	return 1
}

func runPlatformNative(_ context.Context, _ []string, out io.Writer) int {
	return unsupportedPlatformRefusal(out, "platform")
}

func runInfrastructure(_ context.Context, _ []string, out io.Writer) int {
	return unsupportedPlatformRefusal(out, "infrastructure")
}

func runRoute(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "route")
}

func runInstalledEndpoint(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 || !(args[0] == "start-installed" && len(args) == 2 || args[0] == "prepare-history" && len(args) == 4) {
		return 2
	}
	return unsupportedPlatformRefusal(diagnostic, "endpoint")
}

func admissionConsoleFile(*os.File) (*os.File, error) {
	return nil, errors.New("interactive admission requires Linux")
}

func openDocumentCommandIO() (*os.File, *os.File, error) {
	return nil, nil, errors.New("document UI requires the selected Linux host")
}

func runPublishedLink(context.Context, string) error {
	return errors.New("document Link presentation requires the selected Linux host")
}

func writeNetworkPreparationReceipt(context.Context, io.Writer, []byte) error {
	return errors.New("network public preparation unavailable")
}

func openPrivateLinkFile(string) (*os.File, error) {
	return nil, errors.New("private Target Link requires the selected Linux host")
}
