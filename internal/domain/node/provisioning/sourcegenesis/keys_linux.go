package sourcegenesis

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func runSourceInitialKeys(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: ardents infrastructure provision source prepare-initial-keys FRESH_PRIVATE_OUTPUT")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	public, err := prepareSourceInitialKeys(ctx, args[0], nil)
	if err != nil {
		return artifacts.Refuse(err, "source control failed")
	}
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		State     string `json:"state_public_key"`
		Node      string `json:"node_public_key"`
		Accepted  bool   `json:"state_accepted"`
	}{"source.prepare-initial-keys", hex.EncodeToString(public[0][:]), hex.EncodeToString(public[1][:]), false})
	if err != nil {
		return err
	}
	return artifacts.WriteReceipt(ctx, out, append(raw, '\n'))
}

// Fixed two-key offline birth, not a generic key service. It creates no NodeID,
// family, Network identity, accepted State or runtime TLS credential. The caller
// independently selects and reviews those public facts before any signing.
func prepareSourceInitialKeys(ctx context.Context, path string, observe func(string, *os.File)) (public [2][32]byte, result error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || !artifacts.AbsolutePath(path) {
		return public, artifacts.ErrInput
	}
	var files []artifacts.Content
	defer func() {
		for _, file := range files {
			clear(file.Data)
		}
		if result != nil {
			public = [2][32]byte{}
		}
	}()
	for i, name := range []string{"source-state-key.pem", "source-node-key.pem"} {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return public, err
		}
		if i == 1 && bytes.Equal(pub, public[0][:]) {
			clear(key)
			return public, artifacts.ErrInput
		}
		copy(public[i][:], pub)
		der, err := x509.MarshalPKCS8PrivateKey(key)
		clear(key)
		if err != nil {
			return public, err
		}
		raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		clear(der)
		files = append(files, artifacts.Content{Name: name, Data: raw})
	}
	check := func() error {
		if os.Geteuid() != 0 {
			return artifacts.ErrInput
		}
		return ctx.Err()
	}
	result = artifacts.PublishPrivate(ctx, path, files, check, observe)
	return public, result
}
