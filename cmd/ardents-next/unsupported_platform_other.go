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

// Node startup and retained repair refuse before opening the role's plan.
func runNodeForwarding(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.forwarding")
}

func runNodeIssuer(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.issuer")
}

func runNodeDescriptor(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.descriptor")
}

func runNodeIntroduction(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.introduction")
}

func runNodeJoin(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.join")
}

func runSourceCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return unsupportedPlatformRefusal(diagnostic, "source")
	}
	switch args[0] {
	case "prepare-trust":
		return unsupportedPlatformRefusal(out, "source.prepare-trust")
	case "prepare-state", "inspect-latest":
		return unsupportedPlatformRefusal(diagnostic, "source."+args[0])
	case "repair-retained":
		return runRepair(ctx, "source", args[1:], out, diagnostic)
	default:
		return unsupportedPlatformRefusal(diagnostic, "source")
	}
}

func runRepair(_ context.Context, role string, _ []string, out, diagnostic io.Writer) int {
	if role == "source" {
		return unsupportedPlatformRefusal(diagnostic, "source.repair-retained")
	}
	return unsupportedPlatformRefusal(out, "node."+role)
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
