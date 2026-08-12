package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	nodequalification "github.com/dianabuilds/ardents-network/internal/qualification/node"
)

func holdNodeCollector(arguments []string, diagnostics io.Writer) int {
	if len(arguments) != 0 {
		fmt.Fprintln(diagnostics, "collector-node has unexpected arguments")
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return 0
}

func evaluateNodeSample(arguments []string) ([]byte, error) {
	if len(arguments) > 1 {
		return nil, errors.New("sample-node has unexpected arguments")
	}
	if len(arguments) == 1 {
		return nodequalification.SampleHostResources(time.Now(), arguments[0])
	}
	return nodequalification.SampleContainerResources(time.Now())
}

func evaluateNodeBuildIdentity(arguments []string) ([]byte, error) {
	if len(arguments) != 0 {
		return nil, errors.New("build-info-node has unexpected arguments")
	}
	return nodequalification.ReadCandidateBuildIdentity([]string{
		"/usr/local/bin/ardents", "/usr/local/bin/ardents-node", "/usr/local/bin/ardents-qualify",
	})
}
