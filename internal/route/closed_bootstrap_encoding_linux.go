//go:build linux

package route

// EncodeClosedBootstrap returns the lane-zero bootstrap operation body.
func EncodeClosedBootstrap(issuer bool) []byte {
	if issuer {
		return []byte{2}
	}
	return []byte{1}
}
