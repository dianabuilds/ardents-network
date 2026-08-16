package blockedverify

// Config names the immutable verifier inputs and canonical output.
type Config struct {
	ManifestPath    string
	EvidencePath    string
	ClosurePath     string
	SecretRoot      string
	RegistryRoot    string
	CanaryPath      string
	PublishableRoot string
	OutputPath      string
}

// Result is the sole verdict authored by the independent verifier.
type Result struct {
	Schema           string   `json:"schema"`
	Scope            string   `json:"scope"`
	RunID            string   `json:"run_id"`
	Verdict          string   `json:"verdict"`
	Reasons          []string `json:"reasons"`
	ManifestSHA256   string   `json:"manifest_sha256"`
	EvidenceSHA256   string   `json:"evidence_sha256"`
	VerifierSHA256   string   `json:"verifier_sha256"`
	VerifiedUnixNano int64    `json:"verified_unix_nano"`
}
