//go:build !linux

package resource

// MeasureOwnerCgroups requires Linux cgroup v2 and per-process resident pages.
func MeasureOwnerCgroups(workers []string) (Sample, error) {
	return Sample{}, errUnsupportedPlatform
}
