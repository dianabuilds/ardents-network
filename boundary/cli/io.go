package cli

import "io"

func ioReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
