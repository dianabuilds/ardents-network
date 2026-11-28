// Package artifacts owns original Unix-family (Linux and macOS) input
// descriptors and bounded durable publication of operator artifacts. The
// input/directory custody and retained-file identity checks compose the shared
// internal/durablefile inspection port (#850); the protected ext4 storage floor
// (protected_storage_linux.go) and the output publication floor remain genuine
// Linux mechanisms. Directory custody is also used by native Source and
// platform input composition. Its file checks grant no domain authority. The
// caller chooses the artifact set, purpose and signing policy. ReadBoundedFile
// closes its descriptor before returning detached bytes. ObservedInputs retains
// the original descriptor and bytes until Close joins all admitted checks.
package artifacts
