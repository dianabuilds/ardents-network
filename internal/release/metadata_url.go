package release

// MetadataURL returns the only offline-metadata map key accepted by Release
// Decision for one canonical top-level TUF filename.
func MetadataURL(name string) string { return metadataURL(name) }
