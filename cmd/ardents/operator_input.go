package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var errOperatorInputTooLarge = errors.New("operator input exceeds its bound")

// readOperatorInput closes one command-owned input before returning its bounded
// contents. It is deliberately private to ardents: plans are not a shared
// product format or cross-command authority.
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

func decodeOperatorAuthorities(encoded []string, maximum int) (map[[32]byte]ed25519.PublicKey, error) {
	if len(encoded) == 0 || len(encoded) > maximum {
		return nil, errors.New("authority key count is invalid")
	}
	values := make(map[[32]byte]ed25519.PublicKey, len(encoded))
	for _, value := range encoded {
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeOperatorFixedHex(value, public); err != nil {
			return nil, err
		}
		values[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	return values, nil
}

func decodeOperatorDigests(encoded []string, maximum int) ([][32]byte, error) {
	if len(encoded) == 0 || len(encoded) > maximum {
		return nil, errors.New("digest count is invalid")
	}
	values := make([][32]byte, len(encoded))
	for index, value := range encoded {
		if err := decodeOperatorFixedHex(value, values[index][:]); err != nil {
			return nil, err
		}
	}
	return values, nil
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

func freshOperatorRegularFile(path string, clock func() time.Time, maximumAge time.Duration) func() bool {
	return func() bool {
		if path == "" || clock == nil || maximumAge <= 0 {
			return false
		}
		info, err := os.Lstat(path)
		return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 &&
			clock().UTC().Sub(info.ModTime().UTC()).Abs() <= maximumAge
	}
}
