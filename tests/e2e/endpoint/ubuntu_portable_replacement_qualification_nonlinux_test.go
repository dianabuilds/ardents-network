//go:build !linux && endpoint_replacement_qualification

package endpoint_test

import "testing"

func TestUbuntuPortableReplacementQualificationRequiresLinux(t *testing.T) {
	t.Fatal("Endpoint replacement qualification requires an Ubuntu Linux systemd --user host")
}
