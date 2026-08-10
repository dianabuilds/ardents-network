package nativecircuit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/experimentrun"
)

// RunAttached connects two already-listening, owned Unix sockets through the
// authenticated native C-5/C2 Route. The Route sees only opaque stream bytes.
func RunAttached(ctx context.Context, identity experimentrun.Layout, applicationImage, toolImage, userSocket, serviceSocket string, targetRoot, instanceChain, instanceKey []byte) (evidenceDir string, runErr error) {
	_, _, runDirectory, _, err := identity.OwnedPaths(true, true)
	if err != nil {
		return "", err
	}
	attached := &attachedSpec{userSocket: userSocket, serviceSocket: serviceSocket, targetRoot: targetRoot, instanceChain: instanceChain, instanceKey: instanceKey}
	if err := validateAttachedHostSockets(runDirectory, attached); err != nil {
		return "", err
	}
	return runNative(ctx, identity, applicationImage, toolImage, "", nil, attached)
}

func attachedEndpointFixture(attached *attachedSpec) (endpointFixture, error) {
	certificate, err := tls.X509KeyPair(attached.instanceChain, attached.instanceKey)
	if err != nil || len(certificate.Certificate) != 2 {
		return endpointFixture{}, errors.New("attached Instance must supply one leaf and one Target root")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil || leaf.VerifyHostname(endpointServerName) != nil {
		return endpointFixture{}, errors.New("attached Instance leaf is outside the Route TLS contract")
	}
	rootBlock, remainder := pem.Decode(attached.targetRoot)
	if rootBlock == nil || rootBlock.Type != "CERTIFICATE" || len(remainder) != 0 {
		return endpointFixture{}, errors.New("attached Target root is not one canonical certificate")
	}
	root, err := x509.ParseCertificate(rootBlock.Bytes)
	if err != nil || !root.IsCA || leaf.CheckSignatureFrom(root) != nil || !bytes.Equal(certificate.Certificate[1], root.Raw) || !leaf.NotAfter.After(leaf.NotBefore) {
		return endpointFixture{}, errors.New("attached Instance is not issued by the supplied Target root")
	}
	digest := sha256.Sum256(leaf.Raw)
	return endpointFixture{
		rootPEM: attached.targetRoot, chainPEM: attached.instanceChain, privatePEM: attached.instanceKey,
		leafSHA256: hex.EncodeToString(digest[:]), targetMarker: append([]byte(nil), root.RawSubjectPublicKeyInfo...),
	}, nil
}

func ensureNativeDirectory(path string, allowExisting bool) error {
	err := os.Mkdir(path, 0o700)
	if err == nil {
		return nil
	}
	if !allowExisting || !os.IsExist(err) {
		return err
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("shared native run path is not a real directory")
	}
	return nil
}

func validateAttachedHostSockets(runDirectory string, attached *attachedSpec) error {
	if attached == nil || attached.userSocket == attached.serviceSocket {
		return errors.New("attached Route requires two distinct Application sockets")
	}
	for _, path := range []string{attached.userSocket, attached.serviceSocket} {
		if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) != "app.sock" {
			return errors.New("attached Application socket path is not canonical")
		}
		relative, err := filepath.Rel(runDirectory, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("attached Application socket is outside the owned run")
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("attached Application endpoint is not a real Unix socket")
		}
	}
	return nil
}
