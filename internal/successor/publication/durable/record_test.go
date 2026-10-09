package durable

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// Fixture Authority issuance is independent test input, never a runtime signer.
func retainedRecord() ([]byte, [32]byte, [32]byte) {
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{17}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{34}, 32))
	public := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	network := [32]byte{51}
	credential := append([]byte{0, 3}, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, instance.Public().(ed25519.PublicKey)...)
	for _, value := range []uint64{7, 1900000000, 1900003600} {
		credential = binary.BigEndian.AppendUint64(credential, value)
	}
	credential = append(credential, network[:]...)
	credential = binary.BigEndian.AppendUint32(credential, 3)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	record := append([]byte("ardents-service-publication-v3\x00"), credential...)
	record = append(record, bytes.Repeat([]byte{68}, 32)...)
	commitment := sha256.Sum256(record)
	return append(record, ed25519.Sign(instance, commitment[:])...), target, network
}

func TestRetainedPublicRecordHasNoLiveAuthority(t *testing.T) {
	for _, mode := range []string{"retained", "orphan", "wrong-target", "wrong-network", "bad-signature", "wrong-floor", "wrong-pointer"} {
		t.Run(mode, func(t *testing.T) {
			raw, target, network := retainedRecord()
			config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: target, Network: network}
			flush := func(string) error { return nil }
			root, err := openWith(context.Background(), config, flush)
			if err != nil {
				t.Fatal(err)
			}
			root.Close()
			generation := filepath.Join(config.Root, "generations", "0000000000000007")
			if err = os.Mkdir(generation, 0700); err != nil {
				t.Fatal(err)
			}
			floor, pointer := "7\n", "0000000000000007\n"
			switch mode {
			case "wrong-target":
				config.Target[0] ^= 1
			case "wrong-network":
				config.Network[0] ^= 1
			case "bad-signature":
				raw[len(raw)-1] ^= 1
			case "wrong-floor":
				floor = "8\n"
			case "wrong-pointer":
				pointer = "0000000000000008\n"
			}
			for path, contents := range map[string][]byte{filepath.Join(config.Root, "floor"): []byte(floor), filepath.Join(generation, "publication.bin"): raw} {
				if err = os.WriteFile(path, contents, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "orphan" {
				if err = os.WriteFile(filepath.Join(config.Root, "current"), []byte(pointer), 0600); err != nil {
					t.Fatal(err)
				}
			}
			root, err = openWith(context.Background(), config, flush)
			if mode != "retained" && mode != "orphan" {
				if err == nil {
					root.Close()
					t.Fatal("inconsistent retained record accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if floor, err := root.Floor(context.Background()); err != nil || floor != 7 {
				t.Fatal(floor, err)
			}
			if mode == "orphan" {
				if _, err = os.Lstat(generation); !os.IsNotExist(err) {
					t.Fatal("exact unavailable orphan remains", err)
				}
			} else {
				kept, err := os.ReadFile(filepath.Join(generation, "publication.bin"))
				if err != nil || !bytes.Equal(raw, kept) {
					t.Fatal("retained proof changed", err)
				}
			}
		})
	}
}
