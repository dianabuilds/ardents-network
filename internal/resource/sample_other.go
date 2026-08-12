//go:build !linux

package resource

func sampleProcess(profile) (Sample, error) { return Sample{}, nil }
