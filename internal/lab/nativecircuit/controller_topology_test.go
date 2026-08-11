package nativecircuit

import (
	"encoding/json"
	"testing"
)

func TestNativeRoleNetworksAndSidecarNamespacesMustBeExact(t *testing.T) {
	t.Parallel()
	actual := map[string]json.RawMessage{"project_ue-ui": {}, "project_ui-rv": {}, "project_ui-if": {}}
	if !exactNativeRoleNetworks("project", "user-interior", actual, nativeNetworkRoles) {
		t.Fatal("exact User Interior attachments were rejected")
	}
	actual["project_extra"] = json.RawMessage{}
	if exactNativeRoleNetworks("project", "user-interior", actual, nativeNetworkRoles) {
		t.Fatal("extra User Interior attachment was accepted")
	}
	states := map[string]nativeContainerInspect{"shape-user": {}}
	container := states["shape-user"]
	container.HostConfig.NetworkMode = "container:user-id"
	states["shape-user"] = container
	if !exactNativeSidecarNamespaces(states, map[string]string{"user": "user-id"}) {
		t.Fatal("exact User shaper namespace was rejected")
	}
	container.HostConfig.NetworkMode = "container:service-id"
	states["shape-user"] = container
	if exactNativeSidecarNamespaces(states, map[string]string{"user": "user-id"}) {
		t.Fatal("shaper attached to another role namespace was accepted")
	}
}
