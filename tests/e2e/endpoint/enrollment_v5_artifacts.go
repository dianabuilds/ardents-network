package endpoint

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnrollmentV5Artifacts holds the caller-selected predecessor bytes for the
// migration fixture. The native driver enrolls these exact bytes.
type EnrollmentV5Artifacts struct {
	Program []byte
	Custody []byte
}

// ReadEnrollmentV5Artifacts reads the two required programs before staging.
func ReadEnrollmentV5Artifacts(directory string) (EnrollmentV5Artifacts, error) {
	if !filepath.IsAbs(directory) {
		return EnrollmentV5Artifacts{}, fmt.Errorf("an absolute directory with the Linux amd64 v5 Endpoint and Custody artifacts is required")
	}
	read := func(name string) ([]byte, error) {
		raw, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return nil, fmt.Errorf("read predecessor %s: %w", name, err)
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf("predecessor %s is empty", name)
		}
		return raw, nil
	}
	program, err := read("ardents-linux-amd64")
	if err != nil {
		return EnrollmentV5Artifacts{}, err
	}
	custody, err := read("ardents-custody-linux-amd64")
	if err != nil {
		return EnrollmentV5Artifacts{}, err
	}
	return EnrollmentV5Artifacts{Program: program, Custody: custody}, nil
}
