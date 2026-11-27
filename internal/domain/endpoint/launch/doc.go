// Package launch binds the authorized Endpoint generation to its installed
// files and configuration. It stages initial and replacement generations and
// retains independent observations of the admitted runtime invocation.
// The platform service manager owns process start and stop.
//
// StagePlatformInitial and StagePlatformReplacement verify delivery and stage
// the generation. OpenStartup independently retains the original executable,
// configured files and manager at runtime entry. It exposes plans for the
// Endpoint participant, which rechecks its original Startup and closes it
// after joining all application operations.
package launch
