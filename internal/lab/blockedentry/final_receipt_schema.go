package blockedentry

type finalProductReceipt struct {
	SourceSHA256    string `json:"source_sha256"`
	GoArchiveSHA256 string `json:"go_archive_sha256"`
	GoRecipeSHA256  string `json:"go_builder_recipe_sha256"`
	GoModuleSHA256  string `json:"go_module_cache_sha256"`
	RouteSHA256     string `json:"route_sha256"`
	BridgeSHA256    string `json:"bridge_sha256"`
	ServiceSHA256   string `json:"service_sha256"`
	StreamSHA256    string `json:"stream_sha256"`
	PublishSHA256   string `json:"publish_sha256"`
	NetworkSHA256   string `json:"network_test_sha256"`
	AdapterSHA256   string `json:"adapter_test_sha256"`
}

type finalToolReceipt struct {
	BaseDigest     string `json:"base_digest"`
	ToolLockSHA256 string `json:"tool_lock_sha256"`
	SourceSHA256   string `json:"source_sha256"`
	CarrierSHA256  string `json:"carrier_sha256"`
}
