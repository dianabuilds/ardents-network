package installation

// Fixed images are detached bytes. Their construction grants no filesystem,
// Release, selection or startup authority; each native operation retains its
// own admission, original handles, mutation order and physical completion.
// Missing destinations never authorize foreign inode adoption or grant
// platform/Execution admission.
func fixedResourceNames() map[string]string {
	return map[string]string{
		"/usr/lib/ardents/text-worker-root/ardents-text":      "ardents-text-linux-amd64",
		"/etc/systemd/system/ardents-text-reader@.service":    "ardents-text-reader@.service",
		"/etc/systemd/system/ardents-text-publisher@.service": "ardents-text-publisher@.service",
		"/etc/systemd/system/ardents-text-reader.socket":      "ardents-text-reader.socket",
		"/etc/systemd/system/ardents-text-publisher.socket":   "ardents-text-publisher.socket",
		"/usr/share/polkit-1/rules.d/50-ardents-text.rules":   "50-ardents-text.rules",
		"/usr/lib/tmpfiles.d/ardents-text.conf":               "ardents-text.conf",
		"/etc/systemd/system/ardents-endpoint.service":        "ardents-endpoint.service",
	}
}

func fixedResourceImages(files map[string][]byte) (map[string][]byte, error) {
	resources := make(map[string][]byte)
	digests := make(map[string]string)
	for filename, name := range fixedResourceNames() {
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		body := files[name]
		if len(body) == 0 {
			return nil, ErrBinding
		}
		resources[filename] = body
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			digests[filename] = digestHex(body)
		}
	}
	manifest, err := fixedArtifactManifest(digests)
	if err != nil {
		return nil, err
	}
	resources["/etc/ardents/text-worker-artifact.json"] = manifest
	return resources, nil
}

func fixedArtifactManifest(digests map[string]string) ([]byte, error) {
	return canonicalJSON(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", digests})
}
