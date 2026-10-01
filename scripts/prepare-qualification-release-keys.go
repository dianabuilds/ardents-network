//go:build ignore

// Command prepare-qualification-release-keys creates the five operator-owned
// Release keys for ADR-0120. Run only on the selected Linux release host.
// It signs nothing and emits only the public receipt.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/unix"
)

func main() {
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare() error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 || len(os.Args) != 2 {
		return errors.New("requires Linux root and one new absolute private directory")
	}
	root := os.Args[1]
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
		return errors.New("private directory must be a canonical absolute path")
	}
	for parent := filepath.Dir(root); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("private directory ancestor is not a direct directory")
		}
		var stat unix.Stat_t
		if unix.Lstat(parent, &stat) != nil || stat.Uid != 0 || info.Mode().Perm()&0022 != 0 {
			return errors.New("private directory ancestor must be root-owned and not group/other writable")
		}
		if parent == "/" {
			break
		}
	}
	if err := os.Mkdir(root, 0700); err != nil {
		return errors.New("private output directory creation refused; existing output is never replaced")
	}
	// An interrupted output is retained for explicit inspection, never reused.
	public := make([]map[string]string, 0, 5)
	for index := 1; index <= 5; index++ {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return errors.New("release key generation failed")
		}
		encoded, err := x509.MarshalPKCS8PrivateKey(key)
		clear(key)
		if err != nil {
			return errors.New("release key encoding failed")
		}
		private := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
		clear(encoded)
		name := fmt.Sprintf("release-key-%d.pem", index)
		err = writeNew(filepath.Join(root, name), private)
		clear(private)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(pub)
		public = append(public, map[string]string{"file": name, "ed25519_public_hex": hex.EncodeToString(pub), "public_sha256": hex.EncodeToString(digest[:])})
	}
	receipt, err := json.Marshal(struct {
		Schema      string              `json:"schema"`
		Threshold   int                 `json:"ordinary_threshold"`
		Independent bool                `json:"independent_custody"`
		Keys        []map[string]string `json:"keys"`
	}{"ardents-qualification-release-key-receipt-v1", 3, false, public})
	if err != nil {
		return err
	}
	receipt = append(receipt, '\n')
	if err := writeNew(filepath.Join(root, "public-receipt.json"), receipt); err != nil {
		return err
	}
	for _, path := range []string{root, filepath.Dir(root)} {
		dir, err := os.Open(path)
		if err != nil {
			return err
		}
		err = errors.Join(dir.Sync(), dir.Close())
		if err != nil {
			return err
		}
	}
	_, err = os.Stdout.Write(receipt)
	return err
}

func writeNew(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("new release output creation refused")
	}
	n, writeErr := file.Write(contents)
	if writeErr == nil && n != len(contents) {
		writeErr = errors.New("release output write was incomplete")
	}
	return errors.Join(writeErr, file.Sync(), file.Close())
}
