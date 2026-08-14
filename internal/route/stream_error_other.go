//go:build !windows

package route

func platformBenignStreamError(error) bool {
	return false
}
