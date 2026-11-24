package main

import "github.com/dianabuilds/ardents-network/internal/platform/artifacts"

func readBounded(filename string, maximum int64) ([]byte, error) {
	return artifacts.ReadBoundedFile(filename, maximum)
}
