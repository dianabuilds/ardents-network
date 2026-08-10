package nativecircuit

import "testing"

func TestExactCapabilitiesAcceptsDockerNormalizedNames(t *testing.T) {
	var container nativeContainerInspect
	container.Config.User = "0:0"
	container.HostConfig.CapAdd = []string{"CAP_NET_ADMIN"}
	container.HostConfig.CapDrop = []string{"CAP_ALL"}
	if !exactCapabilities(container, "NET_ADMIN") {
		t.Fatal("Docker-normalized capability names must preserve the exact capability contract")
	}
	container.HostConfig.CapAdd = append(container.HostConfig.CapAdd, "CAP_NET_RAW")
	if exactCapabilities(container, "NET_ADMIN") {
		t.Fatal("additional capabilities must be rejected")
	}
}
