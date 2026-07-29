package runtimeimage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidReferenceRequiresCanonicalSHA256Digest(t *testing.T) {
	valid := "registry.example/ardents/node@sha256:" + strings.Repeat("a", 64)
	require.True(t, ValidReference(valid))
	for _, invalid := range []string{
		"", "registry.example/ardents/node:latest",
		"registry.example/Ardents/node@sha256:" + strings.Repeat("a", 64),
		"registry.example/ardents/node@sha512:" + strings.Repeat("a", 128),
	} {
		require.False(t, ValidReference(invalid), invalid)
	}
}
