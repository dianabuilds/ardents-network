package runtimeevent

// Export delivery is independent of a command's result and domain effects.
const (
	ExportLocalOnly   = "local-only"
	ExportCompleted   = "completed"
	ExportUnavailable = "unavailable"
)

// KnownExportDelivery admits only the existing joined exporter classifications.
func KnownExportDelivery(value string) bool {
	return value == ExportLocalOnly || value == ExportCompleted || value == ExportUnavailable
}
