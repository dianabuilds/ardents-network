package blockedentry

// Config fixes one isolated development campaign invocation.
type Config struct {
	WorkspaceRoot      string
	EvidenceRoot       string
	RunID              string
	Mode               string
	RegistryRoot       string
	RunnerPath         string
	VerifierPath       string
	ClientPath         string
	ServerPath         string
	CampaignSpecPath   string
	HostAllocationPath string
	RuntimeComposePath string
	PreparationRoot    string
	ConfigurationRoot  string
	LinuxImage         string
	ImageSHA256        string
	ProductImageID     string
	ToolImageID        string
	GoBuilderImageID   string
	Kernel             string
}

// Result identifies the immutable inputs produced for the independent verifier.
type Result struct {
	ManifestPath    string `json:"manifest_path"`
	EvidencePath    string `json:"evidence_path"`
	ClosurePath     string `json:"closure_path"`
	SecretRoot      string `json:"secret_root"`
	CanaryPath      string `json:"canary_path"`
	PublishableRoot string `json:"publishable_root"`
	ManifestSHA256  string `json:"manifest_sha256"`
	SpecPath        string `json:"spec_path,omitempty"`
	RunnerPath      string `json:"runner_path,omitempty"`
}
