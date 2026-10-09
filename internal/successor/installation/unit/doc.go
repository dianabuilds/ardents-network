// Package unit validates and retains the closed Endpoint template, renders
// caller-admitted write paths, and verifies the fixed installed Endpoint and activation-socket
// contracts against typed systemd observations. Fresh never-started, retained
// quiescent and exact running invocation checks are distinct.
// Failed-attempt termination is a separate cleanup observation; failed state
// remains retained and cannot enter initial or recovery admission.
// It owns expected unit protection and typed invocation grammar, not native observations,
// Installation admission, writable-root selection, process custody or effects.
// Configuration contains detached admitted facts and confers no authority.
package unit
