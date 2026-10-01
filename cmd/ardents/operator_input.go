package main

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var errOperatorInputTooLarge = errors.New("operator input exceeds its bound")

// readOperatorInput closes one command-owned input before returning its bounded
// contents. File access is private to ardents; declaration grammar validation
// can be delegated to its owning module without granting authority.
func readOperatorInput(path string, maximum int64) ([]byte, error) {
	if maximum <= 0 {
		return nil, errors.New("operator input bound must be positive")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	contents, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(contents)) > maximum {
		return contents, errOperatorInputTooLarge
	}
	return contents, nil
}

func decodeOperatorInput(path string, maximum int64, target any) error {
	raw, err := readOperatorInput(path, maximum)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("operator input contains trailing JSON")
	}
	return nil
}

func decodeOperatorFixedHex(encoded string, destination []byte) error {
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != len(destination) {
		return fmt.Errorf("invalid fixed hexadecimal value")
	}
	copy(destination, decoded)
	return nil
}

func readOperatorKeyPair(certificatePath, keyPath string) (tls.Certificate, error) {
	certificate, err := readOperatorInput(certificatePath, 64<<10)
	if err != nil {
		return tls.Certificate{}, err
	}
	key, err := readOperatorInput(keyPath, 64<<10)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certificate, key)
}
