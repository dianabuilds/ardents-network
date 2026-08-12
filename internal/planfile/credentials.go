package planfile

import "crypto/tls"

// KeyPair loads one bounded PEM certificate and private key pair.
func KeyPair(certificatePath, keyPath string) (tls.Certificate, error) {
	certificate, err := Read(certificatePath, 64<<10)
	if err != nil {
		return tls.Certificate{}, err
	}
	key, err := Read(keyPath, 64<<10)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certificate, key)
}
