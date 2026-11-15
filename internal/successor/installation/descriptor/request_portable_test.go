package descriptor

import (
	"strings"
	"testing"
)

func TestPortableRequestDeclaresExactTargetRole(t *testing.T) {
	const raw = `{"schema":"ardents-descriptor-installation-v1","invocation":"descriptor","profile":"ardents-descriptor-process-v1","installation_root":"/opt/ardents-descriptor","state_root":"/var/lib/ardents-state","participation_root":"/var/lib/ardents-participation","spend_root":"/var/lib/ardents-spend","hosting_root":"/var/lib/ardents-hosting","descriptor_root":"/var/lib/ardents-descriptor","credential_directory":"/etc/ardents-node","plan":"/var/lib/ardents-delivery/descriptor.json","certificate":"/var/lib/ardents-delivery/certificate.pem","private_key":"/var/lib/ardents-delivery/private-key.pem","clock":"/var/lib/ardents-clock/observation"}` + "\n"
	if _, err := DecodeRequest([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	foreign := strings.Replace(raw, `"invocation":"descriptor"`, `"invocation":"foreign"`, 1)
	if _, err := DecodeRequest([]byte(foreign)); err == nil {
		t.Fatal("foreign role admitted")
	}
}
