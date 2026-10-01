//go:build linux

package ardp

// EncodeBootstrap returns the lane-zero bootstrap operation body.
func EncodeBootstrap(issuer bool) []byte {
	if issuer {
		return []byte{2}
	}
	return []byte{1}
}
