package config

import (
	"os"
	"strings"
	"time"
)

func classifyChanges(paths []string) (immutable, restart, reloadable []string) {
	for _, path := range paths {
		switch {
		case hasPathPrefix(path, "node.name", "node.data_dir", "network.private_key_path",
			"privacy.capability_store", "privacy.capability_store_key_file", "privacy.replay_key_file",
			"privacy.discovery.replay_path", "privacy.data.replay_path"):
			immutable = append(immutable, path)
		case hasPathPrefix(path, "policy", "logging.level", "diagnostics", "network.discovery_refresh_seconds"):
			reloadable = append(reloadable, path)
		default:
			restart = append(restart, path)
		}
	}
	return immutable, restart, reloadable
}

func hasPathPrefix(path string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+".") {
			return true
		}
	}
	return false
}

func applyReloadableChanges(active, candidate Document) Document {
	next := active
	next.Policy = candidate.Policy
	next.Logging.Level = candidate.Logging.Level
	next.Diagnostics = candidate.Diagnostics
	next.Network.DiscoveryRefreshSeconds = candidate.Network.DiscoveryRefreshSeconds
	return next
}

func safeReason(err error) string {
	if err == nil {
		return ""
	}
	if os.IsNotExist(err) || os.IsPermission(err) {
		return "operator configuration source is unavailable"
	}
	text := err.Error()
	text = redactPathTokens(text)
	if len(text) > 240 {
		return text[:240]
	}
	return text
}

func redactPathTokens(text string) string {
	fields := strings.Fields(text)
	for index, field := range fields {
		trimmed := strings.Trim(field, `"'(),:`)
		if strings.Contains(trimmed, `\`) || strings.HasPrefix(trimmed, "/") {
			fields[index] = strings.Replace(field, trimmed, "[redacted-path]", 1)
		}
	}
	return strings.Join(fields, " ")
}

var nowUTC = func() time.Time { return time.Now().UTC() }
