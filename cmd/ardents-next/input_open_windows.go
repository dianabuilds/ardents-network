package main

import "os"

func openInputFile(filename string) (*os.File, error) {
	// Windows CreateFile does not wait for a named-pipe server to become
	// available. The caller checks the opened type before any Read.
	return os.Open(filename)
}
