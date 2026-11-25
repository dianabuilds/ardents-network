//go:build !linux && !darwin

package hosting

func hostingPlatform() error { return ErrUnsupportedPlatform }

func measureHosting([]string) (hostingReading, error) {
	return hostingReading{}, ErrUnsupportedPlatform
}
