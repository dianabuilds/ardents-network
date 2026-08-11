package architecture

import (
	"bytes"
	"testing"
)

func TestDirectControlPackageHasOneSmallInterface(t *testing.T) {
	t.Parallel()
	assertPackageExports(t, "internal/lab/directcontrol", "RunControl", "RunRole", "RunTamper")
}

func TestDirectTLSControlContract(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	tlsSource := readProjectFile(t, root, "internal/lab/directcontrol/direct_tls.go")
	fixtureSource := readProjectFile(t, root, "internal/lab/directcontrol/control_fixture.go")
	tamperSource := readProjectFile(t, root, "internal/lab/directcontrol/direct_tamper.go")

	for _, required := range []string{
		"tls.VersionTLS13", "tls.X25519", "SessionTicketsDisabled: true",
		`"carrier.invalid"`, `"carrier-lab-direct/1"`,
		"VerifyConnection:", "active Instance leaf does not match", "readDirectRecord",
	} {
		if !bytes.Contains(tlsSource, []byte(required)) {
			t.Errorf("Direct TLS implementation is missing %q", required)
		}
	}
	for _, required := range []string{"ed25519.GenerateKey", "x509.CreateCertificate", "wrong-instance", "active-instance"} {
		if !bytes.Contains(fixtureSource, []byte(required)) {
			t.Errorf("Direct fixture implementation is missing %q", required)
		}
	}
	for _, required := range []string{"protected_record_modified", "payload[len(payload)-1] ^= 0x01"} {
		if !bytes.Contains(tamperSource, []byte(required)) {
			t.Errorf("Direct wire fault implementation is missing %q", required)
		}
	}
}
