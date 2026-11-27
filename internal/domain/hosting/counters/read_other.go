//go:build !linux && !darwin

package counters

func Supported() bool { return false }

// Read returns an observation of the named interfaces and original boot.
func Read([]string) (Reading, error) {
	return Reading{}, ErrUnsupportedPlatform
}
