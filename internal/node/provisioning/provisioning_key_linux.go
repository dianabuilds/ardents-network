package provisioning

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"os"
)

// prepareProvisioningKey generates and durably writes one fresh PKCS#8 key.
// The calling command selects the Node or State filename and owns its purpose.
func prepareProvisioningKey(ctx context.Context, path, name string, observe func(string, *os.File)) (public [32]byte, returned error) {
	defer func() {
		if returned != nil {
			public = [32]byte{}
		}
	}()
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || !artifacts.AbsolutePath(path) || name != "node-key.pem" && name != "state-key.pem" {
		return public, artifacts.ErrInput
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return public, err
	}
	copy(public[:], pub)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	clear(key)
	if err != nil {
		return public, err
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	clear(der)
	defer clear(raw)
	check := func() error {
		if os.Geteuid() != 0 {
			return artifacts.ErrInput
		}
		return ctx.Err()
	}
	if err := artifacts.PublishPrivate(ctx, path, []artifacts.Content{{Name: name, Data: raw}}, check, observe); err != nil {
		return public, err
	}
	return public, check()
}
