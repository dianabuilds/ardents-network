package stage6evidence

type artifact struct {
	Path   string `json:"path"`
	Schema string `json:"schema"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type campaignManifest struct {
	Schema              string     `json:"schema"`
	Profile             string     `json:"profile"`
	RunID               string     `json:"run_id"`
	SourceCommit        string     `json:"source_commit"`
	DirtyDigest         string     `json:"dirty_digest"`
	LauncherSHA256      string     `json:"launcher_sha256"`
	WorkerSHA256        string     `json:"worker_sha256"`
	Platform            string     `json:"platform"`
	Toolchain           string     `json:"toolchain"`
	ClockOrigin         int64      `json:"clock_origin"`
	AdmissionSecretHash string     `json:"admission_secret_sha256"`
	Decisions           []string   `json:"decisions"`
	Cells               []artifact `json:"cells"`
}

type cellManifest struct {
	Schema          string   `json:"schema"`
	ID              string   `json:"id"`
	Ordinal         uint32   `json:"ordinal"`
	Scenario        string   `json:"scenario"`
	ExpectedClass   string   `json:"expected_class"`
	Predicate       string   `json:"predicate"`
	RequiredStreams []string `json:"required_streams"`
}

type evidenceIndex struct {
	Schema         string         `json:"schema"`
	CampaignSHA256 string         `json:"campaign_sha256"`
	Cells          []cellEvidence `json:"cells"`
}

type cellEvidence struct {
	ID             string                `json:"id"`
	Ordinal        uint32                `json:"ordinal"`
	EpisodeOrdinal uint32                `json:"episode_ordinal"`
	Streams        []observationArtifact `json:"streams"`
	TerminalClass  string                `json:"terminal_class"`
	Terminal       artifact              `json:"terminal"`
	Cleanup        artifact              `json:"cleanup"`
}

type observationArtifact struct {
	Path             string `json:"path"`
	Schema           string `json:"schema"`
	Role             string `json:"role"`
	EpisodeOrdinal   uint32 `json:"episode_ordinal"`
	StreamOrdinal    uint32 `json:"stream_ordinal"`
	ObservationStart int64  `json:"observation_start_millis"`
	ObservationEnd   int64  `json:"observation_end_millis"`
	Size             int64  `json:"size"`
	SHA256           string `json:"sha256"`
}

type traceRecord struct {
	Schema      string   `json:"schema"`
	Cell        string   `json:"cell"`
	Ordinal     uint32   `json:"ordinal"`
	Operation   string   `json:"operation"`
	Input       []byte   `json:"input"`
	Output      []byte   `json:"output"`
	Auxiliary   []byte   `json:"auxiliary"`
	Values      []int64  `json:"values"`
	Fields      []string `json:"fields"`
	StartOffset int64    `json:"start_offset_millis"`
	EndOffset   int64    `json:"end_offset_millis"`
}

type terminalRecord struct {
	Schema      string `json:"schema"`
	Cell        string `json:"cell"`
	Ordinal     uint32 `json:"ordinal"`
	Class       string `json:"class"`
	WorkerPID   int64  `json:"worker_pid"`
	WorkerSHA   string `json:"worker_sha256"`
	StartOffset int64  `json:"start_offset_millis"`
	EndOffset   int64  `json:"end_offset_millis"`
}

type cleanupRecord struct {
	Schema    string   `json:"schema"`
	Cell      string   `json:"cell"`
	Ordinal   uint32   `json:"ordinal"`
	Processes []string `json:"processes"`
	Listeners []string `json:"listeners"`
	Temporary []string `json:"temporary"`
}

type cellSpec struct {
	id, scenario, class, predicate string
}
