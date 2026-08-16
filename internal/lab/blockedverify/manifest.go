package blockedverify

import (
	"encoding/hex"
	"os"
	"reflect"
)

var requiredBoundaries = []string{
	"endpoint-adapter", "tls-front", "webtunnel-server", "bridge-next-leg", "publisher-endpoint",
	"ordinary-initiator", "ordinary-introduction", "ordinary-rendezvous", "ordinary-responder",
}
var requiredResiduals = []string{"process", "listener", "socket", "namespace", "mount", "file", "queue", "timer", "cgroup", "publishable-secret"}
var requiredTopology = []topologyRole{
	{"endpoint-application", "application", "app-e", "none"},
	{"endpoint", "endpoint", "endpoint-e", "entry"},
	{"bridge-adapter", "adapter", "adapter-e", "entry"},
	{"bridge-front", "front", "front-b", "bridge"},
	{"bridge-server", "server", "server-b", "bridge-route"},
	{"initiator", "initiator", "route-i", "route"},
	{"introduction", "introduction", "route-x", "route"},
	{"rendezvous", "rendezvous", "route-v", "route"},
	{"responder", "responder", "route-r", "route"},
	{"publisher", "publisher", "endpoint-p", "route"},
	{"publisher-application", "application", "app-p", "none"},
}
var requiredAttributionSources = []attributionSource{
	{"none", "none", "none"},
	{"bridge-adapter", "adapter-e", "candidate"},
	{"evidence-harness", "lab-harness", "harness"},
}

func verifyManifest(value manifest, canaryRaw []byte, executableHash string) []string {
	var reasons []string
	if value.Schema != "ardents-h3-blocked-entry-manifest-v1" || value.CampaignKind != "development-fixture" ||
		value.Profile != developmentFixtureProfile ||
		value.RunID == "" || value.CreatedUnixNano <= 0 {
		reasons = append(reasons, "manifest identity is incomplete or unsupported")
	}
	if value.SourceIdentity != "development-fixture:"+value.HarnessSHA256+":"+value.RunnerSHA256 ||
		value.SupplyClass != "unrestricted-schema-fixture" {
		reasons = append(reasons, "development source or fixture supply identity is invalid")
	}
	if !fixtureModes[value.FixtureMode] {
		reasons = append(reasons, "development fixture mode is unsupported")
	}
	for name, hash := range map[string]string{"harness": value.HarnessSHA256, "runner": value.RunnerSHA256,
		"verifier": value.VerifierSHA256,
		"client":   value.ClientSHA256, "server": value.ServerSHA256, "canary": value.CanarySHA256,
		"nonce": value.ManifestNonceHash, "evidence-root": value.EvidenceRootHash,
		"registry-root": value.RegistryRootHash} {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != 32 {
			reasons = append(reasons, name+" commitment is not SHA-256")
		}
	}
	if value.VerifierSHA256 != executableHash {
		reasons = append(reasons, "manifest does not bind this verifier executable")
	}
	if value.CanarySHA256 != digest(canaryRaw) {
		reasons = append(reasons, "private canary corpus commitment mismatch")
	}
	wanted := hostileMatrix()
	if len(value.Groups) != len(wanted) {
		reasons = append(reasons, "manifest omits a mandatory hostile group")
	} else {
		for index, group := range wanted {
			got := value.Groups[index]
			if got.ID != group.ID || got.Episodes != 5 || !reflect.DeepEqual(got.Variants, group.Variants) {
				reasons = append(reasons, "manifest hostile matrix differs from the verifier contract")
				break
			}
		}
	}
	if !reflect.DeepEqual(value.Boundaries, requiredBoundaries) {
		reasons = append(reasons, "manifest observer boundaries are incomplete or reordered")
	}
	if !reflect.DeepEqual(value.Topology, requiredTopology) {
		reasons = append(reasons, "manifest process and network namespace topology differs from the frozen fixture")
	}
	if !reflect.DeepEqual(value.ResidualKinds, requiredResiduals) {
		reasons = append(reasons, "manifest residual inventory is incomplete or reordered")
	}
	if !reflect.DeepEqual(value.AttributionSources, requiredAttributionSources) {
		reasons = append(reasons, "manifest attribution sources differ from the verifier contract")
	}
	return reasons
}

func verifierExecutableHash() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	hash, _, err := hashFile(path)
	return hash, err
}
