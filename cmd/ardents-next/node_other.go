//go:build !linux

package main

import (
	"context"
	"io"
)

// Node startup and retained repair refuse before opening the role's plan.
func runNodeForwarding(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.forwarding")
}

func runForwardingRepair(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.forwarding")
}

func runNodeIssuer(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.issuer")
}

func runNodeDescriptor(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.descriptor")
}

func runDescriptorRepair(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.descriptor")
}

func runNodeIntroduction(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.introduction")
}

func runIntroductionRepair(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.introduction")
}

func runNodeJoin(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.join")
}

func runJoinRepair(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "node.join")
}
