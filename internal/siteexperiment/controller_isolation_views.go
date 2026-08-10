package siteexperiment

import "encoding/json"

func onlyNoneNetwork(networks map[string]json.RawMessage) bool {
	if len(networks) == 0 {
		return true
	}
	_, found := networks["none"]
	return found && len(networks) == 1
}

func validRoleMounts(role string, mounts []struct {
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}) bool {
	expected := map[string]map[string]bool{
		"http-client":      {"/client": true, "/evidence": true},
		"http-application": {"/service": true, "/evidence": true},
		"client-endpoint":  {"/config": false, "/client": true, "/route": true, "/gateway": false, "/evidence": true},
		"administration":   {"/config": false, "/authority": true, "/evidence": true},
		"authority":        {"/config": false, "/admin": true, "/gateway-authority": true, "/evidence": true},
		"relay":            {"/evidence": true},
		"gateway":          {"/config": false, "/authority": true, "/gateway": true, "/evidence": true},
	}[role]
	if len(expected) != len(mounts) {
		return false
	}
	for _, mount := range mounts {
		writable, ok := expected[mount.Destination]
		if !ok || writable != mount.RW {
			return false
		}
	}
	return true
}
