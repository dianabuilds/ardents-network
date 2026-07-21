package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanRepositoryAllowsConcreteAdapterBoundaries(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/network/waku/startup.go", `package waku

import _ "github.com/waku-org/go-waku/waku/v2/node"
`)
	writeGoFile(t, root, "internal/workload/docker/executor.go", `package docker

import _ "github.com/moby/moby/client"
`)

	findings, err := scanRepository(root)
	require.NoError(t, err)
	require.Empty(t, findings)
}

func TestScanRepositoryReportsForbiddenImports(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "internal/node/node.go", `package node

import (
	_ "github.com/libp2p/go-libp2p"
	_ "github.com/waku-org/go-waku/waku/v2/node"
	_ "github.com/moby/moby/client"
)
`)

	findings, err := scanRepository(root)
	require.NoError(t, err)
	require.Len(t, findings, 3)
	require.Equal(t, "internal/node/node.go", findings[0].File)
}

func TestScanRepositorySkipsThirdPartyAndAimCore(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "third_party/forks/go-waku/sample.go", `package gowaku

import _ "github.com/waku-org/go-waku/waku/v2/node"
`)
	writeGoFile(t, root, "aim-core/internal/transport/sample.go", `package transport

import _ "github.com/libp2p/go-libp2p"
`)

	findings, err := scanRepository(root)
	require.NoError(t, err)
	require.Empty(t, findings)
}

func writeGoFile(t *testing.T, root string, rel string, src string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
}
