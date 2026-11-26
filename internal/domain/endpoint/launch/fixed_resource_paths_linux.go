package launch

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
