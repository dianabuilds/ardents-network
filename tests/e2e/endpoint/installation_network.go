package endpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// InstallationNetwork supplies the fresh host paths, signed Network facts and
// Source observations for a single native Endpoint installation request.
type InstallationNetwork struct {
	Base, InstallationRoot, StateRoot, ClockFile string
	ClientCertificate, ClientKey                 string
	Network, Authority, Seed                     [32]byte
	Addresses                                    [2]string
	SourcePins                                   [2][32]byte
	ObservedAt                                   time.Time
	RefreshMilliseconds                          int
}

// BindInstallationNetwork substitutes original fixture literals in one pass.
// It preserves field order and never searches values supplied by the caller.
func BindInstallationNetwork(t testing.TB, raw string, input InstallationNetwork) string {
	t.Helper()
	quote := func(value string) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	hexValue := func(value [32]byte) string { return quote(hex.EncodeToString(value[:])) }
	replacements := make(map[string]string)
	for original, value := range map[string]string{
		"/installation": input.InstallationRoot, "/state": input.StateRoot,
		"/entry": path.Join(input.Base, "entry"), "/roles": path.Join(input.Base, "roles"),
		"/tokens": path.Join(input.Base, "tokens"), "/clock": input.ClockFile,
		"/socket/reader":          path.Join(input.Base, "application", "reader"),
		"/permissions/request":    path.Join(input.Base, "permission", "request"),
		"/permissions/response":   path.Join(input.Base, "permission", "response"),
		"/credentials/client.pem": input.ClientCertificate, "/credentials/client.key": input.ClientKey,
		"/credentials/a.pem":   path.Join(input.Base, "credentials", "a.pem"),
		"/credentials/b.pem":   path.Join(input.Base, "credentials", "b.pem"),
		"source-a.example:443": input.Addresses[0], "source-b.example:443": input.Addresses[1],
	} {
		replacements[quote(original)] = quote(value)
	}
	for _, field := range []string{"network_authorities", "authority_public"} {
		prefix := quote(field) + ":["
		replacements[prefix+quote(strings.Repeat("ab", 32))+"]"] = prefix + hexValue(input.Authority) + "]"
	}
	for field, binding := range map[string]struct {
		placeholder string
		value       string
	}{
		"network_id":               {strings.Repeat("02", 32), hexValue(input.Network)},
		"closed_profile_authority": {strings.Repeat("ab", 32), hexValue(input.Authority)},
		"order_seed":               {strings.Repeat("05", 32), hexValue(input.Seed)},
		"clock_observed_at":        {"2030-01-02T03:04:05Z", quote(input.ObservedAt.UTC().Format(time.RFC3339))},
	} {
		prefix := quote(field) + ":"
		replacements[prefix+quote(binding.placeholder)] = prefix + binding.value
	}
	for i, host := range [...]string{"source-a.example", "source-b.example"} {
		key := `"identity":`
		replacements[key+quote(strings.Repeat([...]string{"06", "07"}[i], 32))] = key + hexValue(sha256.Sum256([]byte(host)))
	}
	replacements[`"refresh_interval_ms":1000`] = `"refresh_interval_ms":` + strconv.Itoa(input.RefreshMilliseconds)

	leaf := `"leaf_key_digest":` + quote(strings.Repeat("08", 32))
	patterns := make([]string, 0, len(replacements)+1)
	for original := range replacements {
		if !strings.Contains(raw, original) {
			t.Fatalf("installation fixture literal missing: %s", original)
		}
		patterns = append(patterns, regexp.QuoteMeta(original))
	}
	if strings.Count(raw, leaf) != len(input.SourcePins) {
		t.Fatal("installation fixture must declare both original Source pins")
	}
	patterns = append(patterns, regexp.QuoteMeta(leaf))
	nextSource := 0
	return regexp.MustCompile(strings.Join(patterns, "|")).ReplaceAllStringFunc(raw, func(original string) string {
		if original == leaf {
			pin := input.SourcePins[nextSource]
			nextSource++
			return `"leaf_key_digest":` + hexValue(pin)
		}
		return replacements[original]
	})
}
